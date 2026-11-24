package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPaperAccountRepository(t *testing.T) {
	ctx := context.Background()

	dbPath := filepath.Join(t.TempDir(), "paper.db")

	store, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer store.Close()

	account := PaperAccount{
		AccountID:   "test-account",
		Market:      "KRW-BTC",
		Unit:        5,
		InitialCash: 100000,
	}

	if err := store.CreatePaperAccount(ctx, account); err != nil {
		t.Fatalf("CreatePaperAccount failed: %v", err)
	}

	got, err := store.GetPaperAccount(ctx, account.AccountID)
	if err != nil {
		t.Fatalf("GetPaperAccount failed: %v", err)
	}

	if got.Cash != 100000 {
		t.Errorf("expected cash 100000, got %.2f", got.Cash)
	}

	if got.AssetVolume != 0 {
		t.Errorf("expected asset volume 0, got %f", got.AssetVolume)
	}

	got.Cash = 50000
	got.AssetVolume = 0.0005
	got.AvgBuyPrice = 100000000
	got.LastProcessedCandleTime = time.Now().Unix()

	if err := store.UpdatePaperAccount(ctx, *got); err != nil {
		t.Fatalf("UpdatePaperAccount failed: %v", err)
	}

	updated, err := store.GetPaperAccount(ctx, account.AccountID)
	if err != nil {
		t.Fatalf("GetPaperAccount after update failed: %v", err)
	}

	if updated.Cash != 50000 {
		t.Errorf("expected cash 50000, got %.2f", updated.Cash)
	}

	if updated.AssetVolume != 0.0005 {
		t.Errorf("expected asset volume 0.0005, got %f", updated.AssetVolume)
	}
}

func TestPaperTradeRepository(t *testing.T) {
	ctx := context.Background()

	store, err := OpenSQLite(
		ctx,
		filepath.Join(t.TempDir(), "paper.db"),
	)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer store.Close()

	if err := store.CreatePaperAccount(ctx, PaperAccount{
		AccountID:   "test-account",
		Market:      "KRW-BTC",
		Unit:        5,
		InitialCash: 100000,
	}); err != nil {
		t.Fatalf("CreatePaperAccount failed: %v", err)
	}

	trade := PaperTrade{
		AccountID:  "test-account",
		Market:     "KRW-BTC",
		Unit:       5,
		CandleTime: 1790000000,
		Side:       "BUY",
		Price:      100000000,
		Volume:     0.0009,
		Fee:        45,
		Total:      90045,
	}

	if err := store.SavePaperTrade(ctx, trade); err != nil {
		t.Fatalf("SavePaperTrade failed: %v", err)
	}

	trades, err := store.GetPaperTrades(
		ctx,
		"test-account",
		100,
	)
	if err != nil {
		t.Fatalf("GetPaperTrades failed: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	if trades[0].Side != "BUY" {
		t.Errorf("expected BUY, got %s", trades[0].Side)
	}

	if trades[0].Price != 100000000 {
		t.Errorf("unexpected trade price: %f", trades[0].Price)
	}
}

func TestPaperTradeDuplicate(t *testing.T) {
	ctx := context.Background()

	store, err := OpenSQLite(
		ctx,
		filepath.Join(t.TempDir(), "paper.db"),
	)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer store.Close()

	if err := store.CreatePaperAccount(ctx, PaperAccount{
		AccountID:   "test-account",
		Market:      "KRW-BTC",
		Unit:        5,
		InitialCash: 100000,
	}); err != nil {
		t.Fatal(err)
	}

	trade := PaperTrade{
		AccountID:  "test-account",
		Market:     "KRW-BTC",
		Unit:       5,
		CandleTime: 1790000000,
		Side:       "BUY",
		Price:      100000000,
		Volume:     0.0009,
		Total:      90000,
	}

	if err := store.SavePaperTrade(ctx, trade); err != nil {
		t.Fatal(err)
	}

	err = store.SavePaperTrade(ctx, trade)
	if err == nil {
		t.Fatal("expected duplicate trade error")
	}
}

func TestExecutePaperTrade(t *testing.T) {
	ctx := context.Background()

	store, err := OpenSQLite(
		ctx,
		filepath.Join(t.TempDir(), "paper.db"),
	)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer store.Close()

	initial := PaperAccount{
		AccountID:   "transaction-test",
		Market:      "KRW-BTC",
		Unit:        5,
		InitialCash: 100000,
	}

	if err := store.CreatePaperAccount(ctx, initial); err != nil {
		t.Fatal(err)
	}

	const candleTime int64 = 1790000000

	updated := initial
	updated.Cash = 89995
	updated.AssetVolume = 0.0001
	updated.AvgBuyPrice = 100000000
	updated.LastProcessedCandleTime = candleTime

	trade := PaperTrade{
		AccountID:  initial.AccountID,
		Market:     initial.Market,
		Unit:       initial.Unit,
		CandleTime: candleTime,
		Side:       "BUY",
		Price:      100000000,
		Volume:     0.0001,
		Fee:        5,
		Total:      10005,
	}

	if err := store.ExecutePaperTrade(ctx, updated, trade); err != nil {
		t.Fatalf("ExecutePaperTrade failed: %v", err)
	}

	account, err := store.GetPaperAccount(ctx, initial.AccountID)
	if err != nil {
		t.Fatal(err)
	}

	if account.Cash != 89995 {
		t.Errorf("expected cash 89995, got %.2f", account.Cash)
	}

	if account.AssetVolume != 0.0001 {
		t.Errorf("expected asset volume 0.0001, got %f", account.AssetVolume)
	}

	trades, err := store.GetPaperTrades(ctx, initial.AccountID, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
}

func TestExecutePaperTradeRollback(t *testing.T) {
	ctx := context.Background()

	store, err := OpenSQLite(
		ctx,
		filepath.Join(t.TempDir(), "paper.db"),
	)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer store.Close()

	initial := PaperAccount{
		AccountID:   "rollback-test",
		Market:      "KRW-BTC",
		Unit:        5,
		InitialCash: 100000,
	}

	if err := store.CreatePaperAccount(ctx, initial); err != nil {
		t.Fatal(err)
	}

	const candleTime int64 = 1790000000

	trade := PaperTrade{
		AccountID:  initial.AccountID,
		Market:     initial.Market,
		Unit:       initial.Unit,
		CandleTime: candleTime,
		Side:       "BUY",
		Price:      100000000,
		Volume:     0.0001,
		Fee:        5,
		Total:      10005,
	}

	// Insert the same trade first to force a UNIQUE constraint failure.
	if err := store.SavePaperTrade(ctx, trade); err != nil {
		t.Fatal(err)
	}

	updated := initial
	updated.Cash = 89995
	updated.AssetVolume = 0.0001
	updated.AvgBuyPrice = 100000000
	updated.LastProcessedCandleTime = candleTime

	err = store.ExecutePaperTrade(ctx, updated, trade)
	if err == nil {
		t.Fatal("expected transaction failure")
	}

	// The account update must have been rolled back.
	account, err := store.GetPaperAccount(ctx, initial.AccountID)
	if err != nil {
		t.Fatal(err)
	}

	if account.Cash != 100000 {
		t.Errorf("expected cash 100000 after rollback, got %.2f", account.Cash)
	}

	if account.AssetVolume != 0 {
		t.Errorf("expected asset volume 0 after rollback, got %f", account.AssetVolume)
	}

	if account.LastProcessedCandleTime != 0 {
		t.Errorf(
			"expected last processed candle time 0 after rollback, got %d",
			account.LastProcessedCandleTime,
		)
	}

	trades, err := store.GetPaperTrades(ctx, initial.AccountID, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(trades) != 1 {
		t.Errorf("expected original 1 trade, got %d", len(trades))
	}
}
