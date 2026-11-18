package indicator

import "fmt"

// Snapshot represents the latest technical indicators.
type Snapshot struct {
	SMA20 float64
	SMA50 float64
	RSI14 float64
}

// Calculate computes all default indicators.
func Calculate(prices []float64) (Snapshot, error) {

	sma20, err := SMA(prices, 20)
	if err != nil {
		return Snapshot{}, fmt.Errorf("calculate SMA20: %w", err)
	}

	sma50, err := SMA(prices, 50)
	if err != nil {
		return Snapshot{}, fmt.Errorf("calculate SMA50: %w", err)
	}

	rsi14, err := RSI(prices, 14)
	if err != nil {
		return Snapshot{}, fmt.Errorf("calculate RSI14: %w", err)
	}

	return Snapshot{
		SMA20: sma20,
		SMA50: sma50,
		RSI14: rsi14,
	}, nil
}
