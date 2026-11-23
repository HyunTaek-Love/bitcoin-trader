package backtest

import (
	"fmt"
	"math"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/strategy"
)

type Config struct {
	InitialCapital float64
	FeeRate        float64
}

type Trade struct {
	EntryTime  string
	ExitTime   string
	EntryPrice float64
	ExitPrice  float64
	Quantity   float64
	Profit     float64
	ProfitPct  float64
}

type Result struct {
	InitialCapital       float64
	FinalCapital         float64
	TotalReturnPct       float64
	TotalTrades          int
	WinningTrades        int
	WinRatePct           float64
	MaxDrawdownPct       float64
	Trades               []Trade
	BuyHoldFinalCapital  float64
	BuyHoldReturnPct     float64
	AverageWinKRW        float64
	AverageLossKRW       float64
	ProfitFactor         float64
	MaxConsecutiveLosses int
}

func Run(candles []market.Candle, config Config) (Result, error) {
	if len(candles) < strategy.MinimumCandles+1 {
		return Result{}, fmt.Errorf(
			"insufficient candles: required=%d, actual=%d",
			strategy.MinimumCandles+1,
			len(candles),
		)
	}

	if config.InitialCapital <= 0 ||
		math.IsNaN(config.InitialCapital) ||
		math.IsInf(config.InitialCapital, 0) {
		return Result{}, fmt.Errorf("invalid initial capital")
	}

	if config.FeeRate < 0 ||
		config.FeeRate >= 1 ||
		math.IsNaN(config.FeeRate) ||
		math.IsInf(config.FeeRate, 0) {
		return Result{}, fmt.Errorf("invalid fee rate")
	}

	for i, candle := range candles {
		if candle.Time.IsZero() ||
			candle.Open <= 0 ||
			math.IsNaN(candle.Open) ||
			math.IsInf(candle.Open, 0) {
			return Result{}, fmt.Errorf("invalid candle at index %d", i)
		}

		if i > 0 && !candles[i-1].Time.Before(candle.Time) {
			return Result{}, fmt.Errorf(
				"candles must be ordered by time: index=%d",
				i,
			)
		}
	}

	cash := config.InitialCapital
	quantity := 0.0
	entryPrice := 0.0
	entryTime := ""
	entryCapital := 0.0

	result := Result{
		InitialCapital: config.InitialCapital,
		Trades:         make([]Trade, 0),
	}

	peakEquity := config.InitialCapital

	// Evaluate a signal after candle i has closed.
	// Execute the order at candle i+1's opening price.
	for i := strategy.MinimumCandles - 1; i < len(candles)-1; i++ {
		evaluation, err := strategy.Evaluate(candles[:i+1])
		if err != nil {
			return Result{}, fmt.Errorf(
				"evaluate strategy at index %d: %w",
				i,
				err,
			)
		}

		nextCandle := candles[i+1]

		switch evaluation.Action {
		case strategy.Buy:
			if quantity == 0 {
				entryCapital = cash
				entryPrice = nextCandle.Open
				entryTime = nextCandle.Time.Format("2006-01-02 15:04:05")

				// Include the buy-side fee in the purchase cost.
				quantity = cash / (entryPrice * (1 + config.FeeRate))
				cash = 0
			}

		case strategy.Sell:
			if quantity > 0 {
				grossAmount := quantity * nextCandle.Open
				cash = grossAmount * (1 - config.FeeRate)

				profit := cash - entryCapital

				trade := Trade{
					EntryTime:  entryTime,
					ExitTime:   nextCandle.Time.Format("2006-01-02 15:04:05"),
					EntryPrice: entryPrice,
					ExitPrice:  nextCandle.Open,
					Quantity:   quantity,
					Profit:     profit,
					ProfitPct:  profit / entryCapital * 100,
				}

				result.Trades = append(result.Trades, trade)

				if profit > 0 {
					result.WinningTrades++
				}

				quantity = 0
				entryPrice = 0
				entryTime = ""
				entryCapital = 0
			}
		}

		// Calculate equity using the next candle's closing price.
		equity := cash + quantity*nextCandle.Close

		if equity > peakEquity {
			peakEquity = equity
		}

		drawdown := (peakEquity - equity) / peakEquity * 100

		if drawdown > result.MaxDrawdownPct {
			result.MaxDrawdownPct = drawdown
		}
	}

	// Close any remaining position at the last candle's closing price.
	if quantity > 0 {
		lastCandle := candles[len(candles)-1]

		grossAmount := quantity * lastCandle.Close
		cash = grossAmount * (1 - config.FeeRate)

		profit := cash - entryCapital

		trade := Trade{
			EntryTime:  entryTime,
			ExitTime:   lastCandle.Time.Format("2006-01-02 15:04:05"),
			EntryPrice: entryPrice,
			ExitPrice:  lastCandle.Close,
			Quantity:   quantity,
			Profit:     profit,
			ProfitPct:  profit / entryCapital * 100,
		}

		result.Trades = append(result.Trades, trade)

		if profit > 0 {
			result.WinningTrades++
		}

		quantity = 0
	}

	result.FinalCapital = cash
	result.TotalTrades = len(result.Trades)
	result.TotalReturnPct =
		(result.FinalCapital - result.InitialCapital) /
			result.InitialCapital * 100

	if result.TotalTrades > 0 {
		result.WinRatePct =
			float64(result.WinningTrades) /
				float64(result.TotalTrades) * 100
	}

	// Calculate detailed trade statistics.
	calculateTradeStats(&result)

	// Calculate Buy & Hold benchmark.
	result.BuyHoldFinalCapital,
		result.BuyHoldReturnPct =
		calculateBuyAndHold(candles, config)

	return result, nil
}

func calculateTradeStats(result *Result) {
	var totalProfit float64
	var totalLoss float64

	var winningTrades int
	var losingTrades int

	var consecutiveLosses int

	for _, trade := range result.Trades {
		if trade.Profit > 0 {
			totalProfit += trade.Profit
			winningTrades++

			consecutiveLosses = 0
		} else if trade.Profit < 0 {
			totalLoss += trade.Profit
			losingTrades++

			consecutiveLosses++

			if consecutiveLosses > result.MaxConsecutiveLosses {
				result.MaxConsecutiveLosses = consecutiveLosses
			}
		}
	}

	if winningTrades > 0 {
		result.AverageWinKRW =
			totalProfit / float64(winningTrades)
	}

	if losingTrades > 0 {
		result.AverageLossKRW =
			totalLoss / float64(losingTrades)
	}

	if totalLoss < 0 {
		result.ProfitFactor = totalProfit / -totalLoss
	}
}

func calculateBuyAndHold(
	candles []market.Candle,
	config Config,
) (float64, float64) {
	firstPrice := candles[0].Open
	lastPrice := candles[len(candles)-1].Close

	// Buy BTC with the entire initial capital.
	quantity := config.InitialCapital /
		(firstPrice * (1 + config.FeeRate))

	// Sell all BTC at the last candle's closing price.
	finalCapital := quantity *
		lastPrice *
		(1 - config.FeeRate)

	return finalCapital,
		(finalCapital - config.InitialCapital) /
			config.InitialCapital * 100
}
