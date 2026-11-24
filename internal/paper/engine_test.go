package paper

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/storage"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/strategy"
)

func setupPaperEngine(t *testing.T) (*Engine, *storage.SQLiteStore) {
	t.Helper()

	ctx := context.Background()

	store, err := storage.OpenSQLite(
		ctx,
		filepath.Join(t.TempDir(), "paper.db"),
	)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	account := storage.PaperAccount{
		AccountID:   "engine-test",
		Market:      "KRW-BTC",
		Unit:        5,
		InitialCash: 100000,
	}

	if err := store.CreatePaperAccount(ctx, account); err != nil {
		t.Fatalf("CreatePaperAccount failed: %v", err)
	}

	engine, err := NewEngine(store, Config{
		AccountID: account.AccountID,
		FeeRate:   DefaultFeeRate,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	return engine, store
}

func TestNewEngine(t *testing.T) {
	store, err := storage.OpenSQLite(
		context.Background(),
		filepath.Join(t.TempDir(), "paper.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_, err = NewEngine(store, Config{
		AccountID: "test",
		FeeRate:   DefaultFeeRate,
	})
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	_, err = NewEngine(store, Config{
		AccountID: "test",
		FeeRate:   1,
	})
	if err == nil {
		t.Fatal("expected invalid fee rate error")
	}
}

func TestProcessInsufficientCandles(t *testing.T) {
	engine, _ := setupPaperEngine(t)

	_, err := engine.Process(
		context.Background(),
		[]market.Candle{},
	)
	if err == nil {
		t.Fatal("expected insufficient candles error")
	}
}

func TestProcessHoldSignal(t *testing.T) {
	engine, store := setupPaperEngine(t)

	start := time.Date(
		2026, 1, 1, 0, 0, 0, 0, time.UTC,
	)

	candles := make([]market.Candle, 52)

	for i := range candles {
		candles[i] = market.Candle{
			Market: "KRW-BTC",
			Time: start.Add(
				time.Duration(i*5) * time.Minute,
			),
			Open:        100000000,
			High:        101000000,
			Low:         99000000,
			Close:       100000000,
			Volume:      1,
			TradeAmount: 100000000,
		}
	}

	result, err := engine.Process(
		context.Background(),
		candles,
	)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
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

	account, err := store.GetPaperAccount(
		context.Background(),
		"engine-test",
	)
	if err != nil {
		t.Fatal(err)
	}

	if account.Cash != 100000 {
		t.Errorf("expected cash 100000, got %.2f", account.Cash)
	}

	if account.AssetVolume != 0 {
		t.Errorf("expected asset volume 0, got %f", account.AssetVolume)
	}
}
