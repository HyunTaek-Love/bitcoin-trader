package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"
)

var ErrPaperAccountNotFound = errors.New("paper account not found")

// PaperAccount represents a simulated trading account.
type PaperAccount struct {
	AccountID               string
	Market                  string
	Unit                    int
	InitialCash             float64
	Cash                    float64
	AssetVolume             float64
	AvgBuyPrice             float64
	RealizedPnL             float64
	LastProcessedCandleTime int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// PaperTrade represents a simulated trade.
type PaperTrade struct {
	ID          int64
	AccountID   string
	Market      string
	Unit        int
	CandleTime  int64
	Side        string
	Price       float64
	Volume      float64
	Fee         float64
	Total       float64
	RealizedPnL float64
	CreatedAt   time.Time
}

// CreatePaperAccount creates a new simulated trading account.
func (s *SQLiteStore) CreatePaperAccount(
	ctx context.Context,
	account PaperAccount,
) error {
	if account.AccountID == "" {
		return fmt.Errorf("account ID must not be empty")
	}

	if account.Market == "" || account.Unit <= 0 {
		return fmt.Errorf("invalid market or candle unit")
	}

	if !isFiniteNonNegative(account.InitialCash) {
		return fmt.Errorf("invalid initial cash")
	}

	now := time.Now().Unix()

	query := `
	INSERT INTO paper_accounts (
		account_id,
		market,
		unit,
		initial_cash,
		cash,
		asset_volume,
		avg_buy_price,
		realized_pnl,
		last_processed_candle_time,
		created_at,
		updated_at
	)
	VALUES (?, ?, ?, ?, ?, 0, 0, 0, 0, ?, ?)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		account.AccountID,
		account.Market,
		account.Unit,
		account.InitialCash,
		account.InitialCash,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("create paper account: %w", err)
	}

	return nil
}

// GetPaperAccount retrieves a simulated trading account.
func (s *SQLiteStore) GetPaperAccount(
	ctx context.Context,
	accountID string,
) (*PaperAccount, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID must not be empty")
	}

	query := `
	SELECT
		account_id,
		market,
		unit,
		initial_cash,
		cash,
		asset_volume,
		avg_buy_price,
		realized_pnl,
		last_processed_candle_time,
		created_at,
		updated_at
	FROM paper_accounts
	WHERE account_id = ?
	`

	var account PaperAccount
	var createdAt int64
	var updatedAt int64

	err := s.db.QueryRowContext(
		ctx,
		query,
		accountID,
	).Scan(
		&account.AccountID,
		&account.Market,
		&account.Unit,
		&account.InitialCash,
		&account.Cash,
		&account.AssetVolume,
		&account.AvgBuyPrice,
		&account.RealizedPnL,
		&account.LastProcessedCandleTime,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf(
				"%w: %s",
				ErrPaperAccountNotFound,
				accountID,
			)
		}

		return nil, fmt.Errorf("get paper account: %w", err)
	}

	account.CreatedAt = time.Unix(createdAt, 0)
	account.UpdatedAt = time.Unix(updatedAt, 0)

	return &account, nil
}

// UpdatePaperAccount updates the account balance and trading state.
func (s *SQLiteStore) UpdatePaperAccount(
	ctx context.Context,
	account PaperAccount,
) error {
	if account.AccountID == "" {
		return fmt.Errorf("account ID must not be empty")
	}

	values := []float64{
		account.Cash,
		account.AssetVolume,
		account.AvgBuyPrice,
		account.RealizedPnL,
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("account contains invalid numeric value")
		}
	}

	if account.Cash < 0 ||
		account.AssetVolume < 0 ||
		account.AvgBuyPrice < 0 {
		return fmt.Errorf("account balance values must not be negative")
	}

	query := `
	UPDATE paper_accounts
	SET
		cash = ?,
		asset_volume = ?,
		avg_buy_price = ?,
		realized_pnl = ?,
		last_processed_candle_time = ?,
		updated_at = unixepoch()
	WHERE account_id = ?
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		account.Cash,
		account.AssetVolume,
		account.AvgBuyPrice,
		account.RealizedPnL,
		account.LastProcessedCandleTime,
		account.AccountID,
	)
	if err != nil {
		return fmt.Errorf("update paper account: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("paper account not found: %s", account.AccountID)
	}

	return nil
}

// MarkPaperCandleProcessed marks a candle as processed without executing a trade.
// It updates the timestamp only if the candle has not been processed before.
func (s *SQLiteStore) MarkPaperCandleProcessed(
	ctx context.Context,
	accountID string,
	market string,
	unit int,
	candleTime int64,
) (bool, error) {
	if accountID == "" || market == "" {
		return false, fmt.Errorf("account ID and market must not be empty")
	}

	if unit <= 0 || candleTime <= 0 {
		return false, fmt.Errorf("invalid candle unit or time")
	}

	query := `
	UPDATE paper_accounts
	SET
		last_processed_candle_time = ?,
		updated_at = unixepoch()
	WHERE
		account_id = ?
		AND market = ?
		AND unit = ?
		AND last_processed_candle_time < ?
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		candleTime,
		accountID,
		market,
		unit,
		candleTime,
	)
	if err != nil {
		return false, fmt.Errorf("mark paper candle processed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("get affected rows: %w", err)
	}

	return rows == 1, nil
}

// SavePaperTrade saves a simulated trade.
func (s *SQLiteStore) SavePaperTrade(
	ctx context.Context,
	trade PaperTrade,
) error {
	if trade.AccountID == "" || trade.Market == "" {
		return fmt.Errorf("account ID and market must not be empty")
	}

	if trade.Unit <= 0 {
		return fmt.Errorf("unit must be greater than zero")
	}

	if trade.Side != "BUY" && trade.Side != "SELL" {
		return fmt.Errorf("invalid trade side: %s", trade.Side)
	}

	if trade.CandleTime <= 0 {
		return fmt.Errorf("invalid candle time")
	}

	numbers := []float64{
		trade.Price,
		trade.Volume,
		trade.Fee,
		trade.Total,
		trade.RealizedPnL,
	}

	for _, value := range numbers {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("trade contains invalid numeric value")
		}
	}

	if trade.Price <= 0 || trade.Volume <= 0 ||
		trade.Fee < 0 || trade.Total < 0 {
		return fmt.Errorf("invalid trade values")
	}

	query := `
	INSERT INTO paper_trades (
		account_id,
		market,
		unit,
		candle_time,
		side,
		price,
		volume,
		fee,
		total,
		realized_pnl
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		trade.AccountID,
		trade.Market,
		trade.Unit,
		trade.CandleTime,
		trade.Side,
		trade.Price,
		trade.Volume,
		trade.Fee,
		trade.Total,
		trade.RealizedPnL,
	)
	if err != nil {
		return fmt.Errorf("save paper trade: %w", err)
	}

	return nil
}

// GetPaperTrades retrieves simulated trades in chronological order.
func (s *SQLiteStore) GetPaperTrades(
	ctx context.Context,
	accountID string,
	limit int,
) ([]PaperTrade, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID must not be empty")
	}

	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}

	query := `
	SELECT
		id,
		account_id,
		market,
		unit,
		candle_time,
		side,
		price,
		volume,
		fee,
		total,
		realized_pnl,
		created_at
	FROM paper_trades
	WHERE account_id = ?
	ORDER BY candle_time DESC, id DESC
	LIMIT ?
	`

	rows, err := s.db.QueryContext(
		ctx,
		query,
		accountID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query paper trades: %w", err)
	}
	defer rows.Close()

	trades := make([]PaperTrade, 0, limit)

	for rows.Next() {
		var trade PaperTrade
		var createdAt int64

		err := rows.Scan(
			&trade.ID,
			&trade.AccountID,
			&trade.Market,
			&trade.Unit,
			&trade.CandleTime,
			&trade.Side,
			&trade.Price,
			&trade.Volume,
			&trade.Fee,
			&trade.Total,
			&trade.RealizedPnL,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan paper trade: %w", err)
		}

		trade.CreatedAt = time.Unix(createdAt, 0)
		trades = append(trades, trade)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate paper trades: %w", err)
	}

	// Query returns newest first.
	// Reverse to return chronological order.
	for i, j := 0, len(trades)-1; i < j; i, j = i+1, j-1 {
		trades[i], trades[j] = trades[j], trades[i]
	}

	return trades, nil
}

func isFiniteNonNegative(value float64) bool {
	return value >= 0 &&
		!math.IsNaN(value) &&
		!math.IsInf(value, 0)
}

// ExecutePaperTrade atomically updates the account and saves a trade.
func (s *SQLiteStore) ExecutePaperTrade(
	ctx context.Context,
	account PaperAccount,
	trade PaperTrade,
) error {
	if account.AccountID == "" ||
		trade.AccountID == "" ||
		account.AccountID != trade.AccountID {
		return fmt.Errorf("account ID mismatch")
	}

	if account.Market == "" ||
		account.Market != trade.Market ||
		account.Unit <= 0 ||
		account.Unit != trade.Unit {
		return fmt.Errorf("account and trade market configuration mismatch")
	}

	if trade.Side != "BUY" && trade.Side != "SELL" {
		return fmt.Errorf("invalid trade side: %s", trade.Side)
	}

	if trade.CandleTime <= 0 ||
		account.LastProcessedCandleTime != trade.CandleTime {
		return fmt.Errorf("invalid processed candle time")
	}

	accountValues := []float64{
		account.Cash,
		account.AssetVolume,
		account.AvgBuyPrice,
		account.RealizedPnL,
	}

	for _, value := range accountValues {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("account contains invalid numeric value")
		}
	}

	if account.Cash < 0 ||
		account.AssetVolume < 0 ||
		account.AvgBuyPrice < 0 {
		return fmt.Errorf("account balance values must not be negative")
	}

	tradeValues := []float64{
		trade.Price,
		trade.Volume,
		trade.Fee,
		trade.Total,
		trade.RealizedPnL,
	}

	for _, value := range tradeValues {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("trade contains invalid numeric value")
		}
	}

	if trade.Price <= 0 ||
		trade.Volume <= 0 ||
		trade.Fee < 0 ||
		trade.Total < 0 {
		return fmt.Errorf("invalid trade values")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin paper trade transaction: %w", err)
	}
	defer tx.Rollback()

	// Update the account only if this candle has not been processed.
	updateQuery := `
	UPDATE paper_accounts
	SET
		cash = ?,
		asset_volume = ?,
		avg_buy_price = ?,
		realized_pnl = ?,
		last_processed_candle_time = ?,
		updated_at = unixepoch()
	WHERE
		account_id = ?
		AND market = ?
		AND unit = ?
		AND last_processed_candle_time < ?
	`

	result, err := tx.ExecContext(
		ctx,
		updateQuery,
		account.Cash,
		account.AssetVolume,
		account.AvgBuyPrice,
		account.RealizedPnL,
		account.LastProcessedCandleTime,
		account.AccountID,
		account.Market,
		account.Unit,
		trade.CandleTime,
	)
	if err != nil {
		return fmt.Errorf("update paper account: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get updated account rows: %w", err)
	}

	if rows != 1 {
		return fmt.Errorf(
			"account not found or candle already processed: %s",
			account.AccountID,
		)
	}

	// Save the trade in the same transaction.
	insertQuery := `
	INSERT INTO paper_trades (
		account_id,
		market,
		unit,
		candle_time,
		side,
		price,
		volume,
		fee,
		total,
		realized_pnl
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.ExecContext(
		ctx,
		insertQuery,
		trade.AccountID,
		trade.Market,
		trade.Unit,
		trade.CandleTime,
		trade.Side,
		trade.Price,
		trade.Volume,
		trade.Fee,
		trade.Total,
		trade.RealizedPnL,
	)
	if err != nil {
		return fmt.Errorf("insert paper trade: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit paper trade: %w", err)
	}

	return nil
}
