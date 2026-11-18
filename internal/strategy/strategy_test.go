package strategy

import (
	"testing"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
)

func TestEvaluateInsufficientCandles(t *testing.T) {
	candles := makeCandles(50, 100)

	_, err := Evaluate(candles)

	if err == nil {
		t.Fatal("expected error for insufficient candles")
	}
}

func TestEvaluateHold(t *testing.T) {
	candles := makeCandles(60, 100)

	result, err := Evaluate(candles)
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}

	if result.Action != Hold {
		t.Errorf("expected HOLD, got %s", result.Action)
	}
}

func makeCandles(count int, price float64) []market.Candle {
	candles := make([]market.Candle, count)

	start := time.Date(
		2026, 1, 1, 0, 0, 0, 0, time.UTC,
	)

	for i := 0; i < count; i++ {
		candles[i] = market.Candle{
			Market: "KRW-BTC",
			Time:   start.Add(time.Duration(i*5) * time.Minute),
			Open:   price,
			High:   price,
			Low:    price,
			Close:  price,
			Volume: 1,
		}
	}

	return candles
}
