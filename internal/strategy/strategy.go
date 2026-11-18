package strategy

import (
	"fmt"
	"math"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/indicator"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
)

type Action string

const (
	Hold Action = "HOLD"
	Buy  Action = "BUY"
	Sell Action = "SELL"
)

const (
	MinimumCandles = 51
	RSIOverbought  = 70.0
)

// Result contains the result of a strategy evaluation.
type Result struct {
	Action        Action
	Reason        string
	PreviousSMA20 float64
	PreviousSMA50 float64
	SMA20         float64
	SMA50         float64
	RSI14         float64
}

// Evaluate determines whether to buy, sell, or hold.
//
// candles must be ordered from oldest to newest.
// Only completed candles should be provided.
func Evaluate(candles []market.Candle) (Result, error) {
	if len(candles) < MinimumCandles {
		return Result{}, fmt.Errorf(
			"insufficient candles: required=%d, actual=%d",
			MinimumCandles,
			len(candles),
		)
	}

	prices := make([]float64, len(candles))

	for i, candle := range candles {
		if math.IsNaN(candle.Close) ||
			math.IsInf(candle.Close, 0) ||
			candle.Close <= 0 {
			return Result{}, fmt.Errorf(
				"invalid closing price at index %d: %f",
				i,
				candle.Close,
			)
		}

		prices[i] = candle.Close
	}

	previousPrices := prices[:len(prices)-1]

	previousSMA20, err := indicator.SMA(previousPrices, 20)
	if err != nil {
		return Result{}, err
	}

	previousSMA50, err := indicator.SMA(previousPrices, 50)
	if err != nil {
		return Result{}, err
	}

	currentSMA20, err := indicator.SMA(prices, 20)
	if err != nil {
		return Result{}, err
	}

	currentSMA50, err := indicator.SMA(prices, 50)
	if err != nil {
		return Result{}, err
	}

	rsi14, err := indicator.RSI(prices, 14)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Action:        Hold,
		Reason:        "No moving average crossover",
		PreviousSMA20: previousSMA20,
		PreviousSMA50: previousSMA50,
		SMA20:         currentSMA20,
		SMA50:         currentSMA50,
		RSI14:         rsi14,
	}

	// Golden cross: SMA20 crosses above SMA50.
	if previousSMA20 <= previousSMA50 &&
		currentSMA20 > currentSMA50 {

		if rsi14 >= RSIOverbought {
			result.Reason = "Golden cross detected, but RSI is overbought"
			return result, nil
		}

		result.Action = Buy
		result.Reason = "Golden cross detected and RSI is below 70"

		return result, nil
	}

	// Dead cross: SMA20 crosses below SMA50.
	if previousSMA20 >= previousSMA50 &&
		currentSMA20 < currentSMA50 {

		result.Action = Sell
		result.Reason = "Dead cross detected"

		return result, nil
	}

	return result, nil
}
