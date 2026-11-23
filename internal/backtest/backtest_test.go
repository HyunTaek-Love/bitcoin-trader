package backtest

import (
	"testing"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
)

func TestRunInsufficientCandles(t *testing.T) {
	candles := makeTestCandles(50)

	_, err := Run(candles, Config{
		InitialCapital: 100000,
		FeeRate:        0.0005,
	})

	if err == nil {
		t.Fatal("expected error for insufficient candles")
	}
}

func TestRunHoldStrategy(t *testing.T) {
	candles := makeTestCandles(100)

	result, err := Run(candles, Config{
		InitialCapital: 100000,
		FeeRate:        0.0005,
	})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.TotalTrades != 0 {
		t.Errorf("expected 0 trades, got %d", result.TotalTrades)
	}

	if result.FinalCapital != 100000 {
		t.Errorf(
			"expected final capital 100000, got %.2f",
			result.FinalCapital,
		)
	}

	if result.TotalReturnPct != 0 {
		t.Errorf(
			"expected 0 return, got %.4f",
			result.TotalReturnPct,
		)
	}
}

func TestRunInvalidFeeRate(t *testing.T) {
	candles := makeTestCandles(100)

	_, err := Run(candles, Config{
		InitialCapital: 100000,
		FeeRate:        1.0,
	})

	if err == nil {
		t.Fatal("expected error for invalid fee rate")
	}
}

func makeTestCandles(count int) []market.Candle {
	candles := make([]market.Candle, count)

	start := time.Date(
		2026, 1, 1, 0, 0, 0, 0, time.UTC,
	)

	for i := 0; i < count; i++ {
		candles[i] = market.Candle{
			Market: "KRW-BTC",
			Time:   start.Add(time.Duration(i*5) * time.Minute),
			Open:   100000000,
			High:   100000000,
			Low:    100000000,
			Close:  100000000,
			Volume: 1,
		}
	}

	return candles
}
