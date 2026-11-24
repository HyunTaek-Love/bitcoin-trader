package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SQLiteStore manages the SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLite opens or creates a SQLite database.
func OpenSQLite(ctx context.Context, dbPath string) (*SQLiteStore, error) {
	dir := filepath.Dir(dbPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite connection settings.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	}

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure database: %w", err)
		}
	}

	store := &SQLiteStore{
		db: db,
	}

	if err := store.createTables(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// createTables creates the required database tables.

func (s *SQLiteStore) createTables(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS candles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		market TEXT NOT NULL,
		unit INTEGER NOT NULL,
		candle_time INTEGER NOT NULL,
		open REAL NOT NULL,
		high REAL NOT NULL,
		low REAL NOT NULL,
		close REAL NOT NULL,
		volume REAL NOT NULL,
		trade_amount REAL NOT NULL,
		created_at INTEGER NOT NULL DEFAULT (unixepoch()),
		updated_at INTEGER NOT NULL DEFAULT (unixepoch()),

		UNIQUE(market, unit, candle_time)
	);

	CREATE INDEX IF NOT EXISTS idx_candles_market_time
	ON candles(market, unit, candle_time);

	CREATE TABLE IF NOT EXISTS paper_accounts (
		account_id TEXT PRIMARY KEY,
		market TEXT NOT NULL,
		unit INTEGER NOT NULL,
		initial_cash REAL NOT NULL CHECK(initial_cash >= 0),
		cash REAL NOT NULL CHECK(cash >= 0),
		asset_volume REAL NOT NULL DEFAULT 0 CHECK(asset_volume >= 0),
		avg_buy_price REAL NOT NULL DEFAULT 0,
		realized_pnl REAL NOT NULL DEFAULT 0,
		last_processed_candle_time INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL DEFAULT (unixepoch()),
		updated_at INTEGER NOT NULL DEFAULT (unixepoch())
	);

	CREATE TABLE IF NOT EXISTS paper_trades (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id TEXT NOT NULL,
		market TEXT NOT NULL,
		unit INTEGER NOT NULL,
		candle_time INTEGER NOT NULL,
		side TEXT NOT NULL CHECK(side IN ('BUY', 'SELL')),
		price REAL NOT NULL CHECK(price > 0),
		volume REAL NOT NULL CHECK(volume > 0),
		fee REAL NOT NULL DEFAULT 0,
		total REAL NOT NULL,
		realized_pnl REAL NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL DEFAULT (unixepoch()),

		FOREIGN KEY(account_id)
			REFERENCES paper_accounts(account_id),

		UNIQUE(account_id, candle_time, side)
	);

	CREATE INDEX IF NOT EXISTS idx_paper_trades_account_time
	ON paper_trades(account_id, candle_time);
	`

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
