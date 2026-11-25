package paper

import (
	"context"
	"testing"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/strategy"
)

func TestStreamProcessor(t *testing.T) {
	engine, store := setupPaperEngine(t)
	ctx := context.Background()

	processor, err := NewStreamProcessor(
		store,
		engine,
		"KRW-BTC",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(
		2026, 1, 1, 0, 0, 0, 0, time.UTC,
	)

	// Prepare 51 completed candles.
	history := make([]market.Candle, strategy.MinimumCandles)

	for i := range history {
		history[i] = market.Candle{
			Market:      "KRW-BTC",
			Time:        start.Add(time.Duration(i*5) * time.Minute),
			Open:        100000000,
			High:        101000000,
			Low:         99000000,
			Close:       100000000,
			Volume:      1,
			TradeAmount: 100000000,
		}
	}

	if _, err := store.SaveCandles(ctx, 5, history); err != nil {
		t.Fatal(err)
	}

	firstRealtimeCandle := market.Candle{
		Market: "KRW-BTC",
		Time: start.Add(
			time.Duration(strategy.MinimumCandles*5) * time.Minute,
		),
		Open:        100000000,
		High:        101000000,
		Low:         99000000,
		Close:       100000000,
		Volume:      1,
		TradeAmount: 100000000,
	}

	// First message initializes the processor.
	result, err := processor.HandleCandle(
		ctx,
		firstRealtimeCandle,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatal("first candle must not trigger evaluation")
	}

	// Another update of the same candle must not trigger evaluation.
	firstRealtimeCandle.Close = 100100000

	result, err = processor.HandleCandle(
		ctx,
		firstRealtimeCandle,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatal("same candle update must not trigger evaluation")
	}

	// A new candle starts.
	nextCandle := market.Candle{
		Market:      "KRW-BTC",
		Time:        firstRealtimeCandle.Time.Add(5 * time.Minute),
		Open:        100100000,
		High:        101000000,
		Low:         99000000,
		Close:       100100000,
		Volume:      1,
		TradeAmount: 100000000,
	}

	result, err = processor.HandleCandle(ctx, nextCandle)
	if err != nil {
		t.Fatalf("HandleCandle failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected strategy evaluation on new candle")
	}

	if !result.Evaluated {
		t.Fatal("expected strategy to be evaluated")
	}

	if result.Action != strategy.Hold {
		t.Errorf("expected HOLD, got %s", result.Action)
	}

	if result.Executed {
		t.Fatal("HOLD signal must not execute a trade")
	}

	account, err := store.GetPaperAccount(ctx, "engine-test")
	if err != nil {
		t.Fatal(err)
	}

	if account.Cash != 100000 {
		t.Errorf("expected cash 100000, got %.2f", account.Cash)
	}
}

func TestStreamProcessorSkipsMissedCandle(t *testing.T) {
	engine, store := setupPaperEngine(t)
	ctx := context.Background()

	processor, err := NewStreamProcessor(
		store,
		engine,
		"KRW-BTC",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(
		2026, 1, 1, 0, 0, 0, 0, time.UTC,
	)

	first := market.Candle{
		Market: "KRW-BTC",
		Time:   start,
		Open:   100000000,
		Close:  100000000,
	}

	if _, err := processor.HandleCandle(ctx, first); err != nil {
		t.Fatal(err)
	}

	// A 10-minute jump means one 5-minute candle was missed.
	missed := first
	missed.Time = start.Add(10 * time.Minute)

	result, err := processor.HandleCandle(ctx, missed)
	if err != nil {
		t.Fatal(err)
	}

	if result != nil {
		t.Fatal("must not evaluate after missing a candle")
	}
}

func TestStreamProcessorIgnoresOlderCandle(t *testing.T) {
	engine, store := setupPaperEngine(t)
	ctx := context.Background()

	processor, err := NewStreamProcessor(
		store,
		engine,
		"KRW-BTC",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(
		2026, 1, 1, 0, 0, 0, 0, time.UTC,
	)

	original := market.Candle{
		Market: "KRW-BTC",
		Time:   start,
		Open:   100000000,
		High:   101000000,
		Low:    99000000,
		Close:  100000000,
		Volume: 1,
	}

	if _, err := store.SaveCandles(
		ctx,
		5,
		[]market.Candle{original},
	); err != nil {
		t.Fatal(err)
	}

	current := original
	current.Time = start.Add(5 * time.Minute)

	if _, err := processor.HandleCandle(
		ctx,
		current,
	); err != nil {
		t.Fatal(err)
	}

	// Send a delayed candle with a different closing price.
	delayed := original
	delayed.Close = 90000000

	if _, err := processor.HandleCandle(
		ctx,
		delayed,
	); err != nil {
		t.Fatal(err)
	}

	candles, err := store.GetCandles(
		ctx,
		"KRW-BTC",
		5,
		2,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(candles) != 2 {
		t.Fatalf(
			"expected 2 candles, got %d",
			len(candles),
		)
	}

	if candles[0].Close != original.Close {
		t.Errorf(
			"historical candle was overwritten: expected=%.0f actual=%.0f",
			original.Close,
			candles[0].Close,
		)
	}
}

func TestStreamProcessorRecoversAfterGap(t *testing.T) {
	engine, store := setupPaperEngine(t)
	ctx := context.Background()

	processor, err := NewStreamProcessor(
		store,
		engine,
		"KRW-BTC",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(
		2026, 1, 1, 0, 0, 0, 0, time.UTC,
	)

	history := make([]market.Candle, strategy.MinimumCandles)

	for i := range history {
		history[i] = market.Candle{
			Market:      "KRW-BTC",
			Time:        start.Add(time.Duration(i*5) * time.Minute),
			Open:        100000000,
			High:        101000000,
			Low:         99000000,
			Close:       100000000,
			Volume:      1,
			TradeAmount: 100000000,
		}
	}

	if _, err := store.SaveCandles(ctx, 5, history); err != nil {
		t.Fatal(err)
	}

	// First WebSocket candle establishes the baseline.
	firstRealtime := market.Candle{
		Market: "KRW-BTC",
		Time: start.Add(
			time.Duration(strategy.MinimumCandles*5) * time.Minute,
		),
		Open:   100000000,
		High:   101000000,
		Low:    99000000,
		Close:  100000000,
		Volume: 1,
	}

	if _, err := processor.HandleCandle(ctx, firstRealtime); err != nil {
		t.Fatal(err)
	}

	// Simulate REST recovery of the candle missed by WebSocket.
	recovered := firstRealtime
	recovered.Time = firstRealtime.Time.Add(5 * time.Minute)

	if _, err := store.SaveCandles(
		ctx,
		5,
		[]market.Candle{recovered},
	); err != nil {
		t.Fatal(err)
	}

	// WebSocket reconnects and sends the current candle.
	current := recovered
	current.Time = recovered.Time.Add(5 * time.Minute)

	result, err := processor.HandleCandle(ctx, current)
	if err != nil {
		t.Fatalf("HandleCandle failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected strategy evaluation after recovery")
	}

	if !result.Evaluated {
		t.Fatal("expected strategy to be evaluated")
	}

	if result.Executed {
		t.Fatal("HOLD signal must not execute a trade")
	}
}

func TestStreamProcessorSkipsWhenRecoveryHasGap(t *testing.T) {
	engine, store := setupPaperEngine(t)
	ctx := context.Background()

	processor, err := NewStreamProcessor(
		store,
		engine,
		"KRW-BTC",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Date(
		2026, 1, 1, 0, 0, 0, 0, time.UTC,
	)

	history := make([]market.Candle, strategy.MinimumCandles)

	for i := range history {
		history[i] = market.Candle{
			Market: "KRW-BTC",
			Time: start.Add(
				time.Duration(i*5) * time.Minute,
			),
			Open:   100000000,
			High:   101000000,
			Low:    99000000,
			Close:  100000000,
			Volume: 1,
		}
	}

	if _, err := store.SaveCandles(ctx, 5, history); err != nil {
		t.Fatal(err)
	}

	firstRealtime := market.Candle{
		Market: "KRW-BTC",
		Time: start.Add(
			time.Duration(strategy.MinimumCandles*5) * time.Minute,
		),
		Open:   100000000,
		High:   101000000,
		Low:    99000000,
		Close:  100000000,
		Volume: 1,
	}

	if _, err := processor.HandleCandle(ctx, firstRealtime); err != nil {
		t.Fatal(err)
	}

	// The next 5-minute candle is missing from SQLite.
	// The current candle arrives 10 minutes later.
	current := firstRealtime
	current.Time = firstRealtime.Time.Add(10 * time.Minute)

	result, err := processor.HandleCandle(ctx, current)
	if err != nil {
		t.Fatal(err)
	}

	if result != nil {
		t.Fatal("must skip evaluation when recovered history has a gap")
	}
}
