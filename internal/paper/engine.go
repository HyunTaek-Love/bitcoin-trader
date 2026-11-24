package paper

import (
	"context"
	"fmt"
	"math"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/storage"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/strategy"
)

const DefaultFeeRate = 0.0005

type Config struct {
	AccountID string
	FeeRate   float64
}

type Engine struct {
	store  *storage.SQLiteStore
	config Config
}

type ProcessResult struct {
	Evaluated bool
	Action    strategy.Action
	Reason    string
	Executed  bool
	Trade     *storage.PaperTrade
}

// NewEngine creates a paper trading engine.
func NewEngine(
	store *storage.SQLiteStore,
	config Config,
) (*Engine, error) {
	if store == nil {
		return nil, fmt.Errorf("storage must not be nil")
	}

	if config.AccountID == "" {
		return nil, fmt.Errorf("account ID must not be empty")
	}

	if config.FeeRate < 0 ||
		config.FeeRate >= 1 ||
		math.IsNaN(config.FeeRate) ||
		math.IsInf(config.FeeRate, 0) {
		return nil, fmt.Errorf("invalid fee rate")
	}

	return &Engine{
		store:  store,
		config: config,
	}, nil
}

// Process evaluates completed candles and executes a signal
// at the opening price of the newest candle.
//
// candles must be chronological.
// The newest candle represents the newly started execution candle.
// All preceding candles must be completed.
func (e *Engine) Process(
	ctx context.Context,
	candles []market.Candle,
) (ProcessResult, error) {
	if len(candles) < strategy.MinimumCandles+1 {
		return ProcessResult{}, fmt.Errorf(
			"insufficient candles: required=%d, actual=%d",
			strategy.MinimumCandles+1,
			len(candles),
		)
	}

	executionCandle := candles[len(candles)-1]
	signalCandles := candles[:len(candles)-1]

	if executionCandle.Time.IsZero() ||
		executionCandle.Open <= 0 ||
		math.IsNaN(executionCandle.Open) ||
		math.IsInf(executionCandle.Open, 0) {
		return ProcessResult{}, fmt.Errorf("invalid execution candle")
	}

	if executionCandle.Market == "" {
		return ProcessResult{}, fmt.Errorf("execution candle market is empty")
	}

	for i, candle := range signalCandles {
		if candle.Market != executionCandle.Market {
			return ProcessResult{}, fmt.Errorf(
				"market mismatch at candle index %d",
				i,
			)
		}

		if candle.Time.IsZero() ||
			(i > 0 && !signalCandles[i-1].Time.Before(candle.Time)) {
			return ProcessResult{}, fmt.Errorf(
				"invalid candle order at index %d",
				i,
			)
		}
	}

	lastSignalCandle := signalCandles[len(signalCandles)-1]

	if !lastSignalCandle.Time.Before(executionCandle.Time) {
		return ProcessResult{}, fmt.Errorf(
			"execution candle must be newer than signal candles",
		)
	}

	account, err := e.store.GetPaperAccount(
		ctx,
		e.config.AccountID,
	)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("load paper account: %w", err)
	}

	if account.Market != executionCandle.Market {
		return ProcessResult{}, fmt.Errorf(
			"account market mismatch: account=%s candle=%s",
			account.Market,
			executionCandle.Market,
		)
	}

	if account.Unit <= 0 {
		return ProcessResult{}, fmt.Errorf("invalid account candle unit")
	}

	result := ProcessResult{
		Evaluated: true,
		Action:    strategy.Hold,
	}

	executionTime := executionCandle.Time.Unix()

	// Prevent processing an execution candle more than once.
	if executionTime <= account.LastProcessedCandleTime {
		result.Reason = "Execution candle already processed"
		return result, nil
	}

	evaluation, err := strategy.Evaluate(signalCandles)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("evaluate strategy: %w", err)
	}

	result.Action = evaluation.Action
	result.Reason = evaluation.Reason

	switch evaluation.Action {
	case strategy.Buy:
		if account.AssetVolume > 0 {
			result.Reason = "Buy signal ignored: position already exists"
			return result, nil
		}

		if account.Cash <= 0 {
			result.Reason = "Buy signal ignored: insufficient cash"
			return result, nil
		}

		trade, updatedAccount, err := e.calculateBuy(
			*account,
			executionCandle,
		)
		if err != nil {
			return result, err
		}

		if err := e.store.ExecutePaperTrade(
			ctx,
			updatedAccount,
			trade,
		); err != nil {
			return result, fmt.Errorf("execute paper buy: %w", err)
		}

		result.Executed = true
		result.Trade = &trade

	case strategy.Sell:
		if account.AssetVolume <= 0 {
			result.Reason = "Sell signal ignored: no position"
			return result, nil
		}

		trade, updatedAccount, err := e.calculateSell(
			*account,
			executionCandle,
		)
		if err != nil {
			return result, err
		}

		if err := e.store.ExecutePaperTrade(
			ctx,
			updatedAccount,
			trade,
		); err != nil {
			return result, fmt.Errorf("execute paper sell: %w", err)
		}

		result.Executed = true
		result.Trade = &trade

	default:
		result.Reason = evaluation.Reason
	}

	return result, nil
}

func (e *Engine) calculateBuy(
	account storage.PaperAccount,
	candle market.Candle,
) (storage.PaperTrade, storage.PaperAccount, error) {
	price := candle.Open

	// Spend all available cash, including the trading fee.
	grossAmount := account.Cash / (1 + e.config.FeeRate)
	fee := grossAmount * e.config.FeeRate
	volume := grossAmount / price

	if volume <= 0 ||
		math.IsNaN(volume) ||
		math.IsInf(volume, 0) {
		return storage.PaperTrade{}, account,
			fmt.Errorf("invalid buy volume")
	}

	totalCost := grossAmount + fee

	newVolume := account.AssetVolume + volume
	if newVolume <= 0 {
		return storage.PaperTrade{}, account,
			fmt.Errorf("invalid resulting asset volume")
	}

	newAveragePrice := (account.AssetVolume*account.AvgBuyPrice + totalCost) / newVolume

	updated := account
	updated.Cash = math.Max(0, account.Cash-totalCost)
	updated.AssetVolume = newVolume
	updated.AvgBuyPrice = newAveragePrice
	updated.LastProcessedCandleTime = candle.Time.Unix()

	trade := storage.PaperTrade{
		AccountID:  account.AccountID,
		Market:     account.Market,
		Unit:       account.Unit,
		CandleTime: candle.Time.Unix(),
		Side:       "BUY",
		Price:      price,
		Volume:     volume,
		Fee:        fee,
		Total:      grossAmount,
	}

	return trade, updated, nil
}

func (e *Engine) calculateSell(
	account storage.PaperAccount,
	candle market.Candle,
) (storage.PaperTrade, storage.PaperAccount, error) {
	price := candle.Open
	volume := account.AssetVolume

	grossAmount := price * volume
	fee := grossAmount * e.config.FeeRate
	netAmount := grossAmount - fee

	realizedPnL := netAmount -
		(account.AvgBuyPrice * volume)

	updated := account
	updated.Cash += netAmount
	updated.AssetVolume = 0
	updated.AvgBuyPrice = 0
	updated.RealizedPnL += realizedPnL
	updated.LastProcessedCandleTime = candle.Time.Unix()

	trade := storage.PaperTrade{
		AccountID:   account.AccountID,
		Market:      account.Market,
		Unit:        account.Unit,
		CandleTime:  candle.Time.Unix(),
		Side:        "SELL",
		Price:       price,
		Volume:      volume,
		Fee:         fee,
		Total:       grossAmount,
		RealizedPnL: realizedPnL,
	}

	return trade, updated, nil
}
