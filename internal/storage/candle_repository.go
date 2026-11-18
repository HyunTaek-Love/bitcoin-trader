package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
)

// SaveCandles saves candles to the database.
// Existing candles are updated when the unique key already exists.
func (s *SQLiteStore) SaveCandles(
	ctx context.Context,
	unit int,
	candles []market.Candle,
) (int64, error) {

	if unit <= 0 {
		return 0, fmt.Errorf("unit must be greater than zero")
	}

	if len(candles) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
	INSERT INTO candles (
		market,
		unit,
		candle_time,
		open,
		high,
		low,
		close,
		volume,
		trade_amount,
		updated_at
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch())
	ON CONFLICT(market, unit, candle_time)
	DO UPDATE SET
		open = excluded.open,
		high = excluded.high,
		low = excluded.low,
		close = excluded.close,
		volume = excluded.volume,
		trade_amount = excluded.trade_amount,
		updated_at = unixepoch()
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("prepare insert statement: %w", err)
	}
	defer stmt.Close()

	var affected int64

	for _, candle := range candles {
		if candle.Market == "" || candle.Time.IsZero() {
			return 0, fmt.Errorf("invalid candle data")
		}

		result, err := stmt.ExecContext(
			ctx,
			candle.Market,
			unit,
			candle.Time.Unix(),
			candle.Open,
			candle.High,
			candle.Low,
			candle.Close,
			candle.Volume,
			candle.TradeAmount,
		)
		if err != nil {
			return 0, fmt.Errorf("save candle: %w", err)
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("get affected rows: %w", err)
		}

		affected += rows
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return affected, nil
}

// GetCandles retrieves candles in chronological order.
func (s *SQLiteStore) GetCandles(
	ctx context.Context,
	marketCode string,
	unit int,
	limit int,
) ([]market.Candle, error) {

	if marketCode == "" {
		return nil, fmt.Errorf("market code must not be empty")
	}

	if unit <= 0 || limit <= 0 {
		return nil, fmt.Errorf("unit and limit must be greater than zero")
	}

	query := `
	SELECT
		market,
		candle_time,
		open,
		high,
		low,
		close,
		volume,
		trade_amount
	FROM candles
	WHERE market = ? AND unit = ?
	ORDER BY candle_time DESC
	LIMIT ?
	`

	rows, err := s.db.QueryContext(
		ctx,
		query,
		marketCode,
		unit,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query candles: %w", err)
	}
	defer rows.Close()

	candles := make([]market.Candle, 0, limit)

	for rows.Next() {
		var candle market.Candle
		var candleTime int64

		err := rows.Scan(
			&candle.Market,
			&candleTime,
			&candle.Open,
			&candle.High,
			&candle.Low,
			&candle.Close,
			&candle.Volume,
			&candle.TradeAmount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan candle: %w", err)
		}

		candle.Time = time.Unix(candleTime, 0).UTC()
		candles = append(candles, candle)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candles: %w", err)
	}

	// Query returns newest first.
	// Reverse to return oldest first.
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}

	return candles, nil
}
