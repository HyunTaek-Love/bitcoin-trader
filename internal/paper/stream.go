package paper

import (
	"context"
	"fmt"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/storage"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/strategy"
)

type StreamProcessor struct {
	store  *storage.SQLiteStore
	engine *Engine
	market string
	unit   int

	initialized       bool
	currentCandleTime time.Time
}

func NewStreamProcessor(
	store *storage.SQLiteStore,
	engine *Engine,
	marketCode string,
	unit int,
) (*StreamProcessor, error) {
	if store == nil {
		return nil, fmt.Errorf("storage must not be nil")
	}

	if engine == nil {
		return nil, fmt.Errorf("engine must not be nil")
	}

	if marketCode == "" || unit <= 0 {
		return nil, fmt.Errorf("invalid market or candle unit")
	}

	return &StreamProcessor{
		store:  store,
		engine: engine,
		market: marketCode,
		unit:   unit,
	}, nil
}

// HandleCandle receives a realtime candle update.
// It evaluates the strategy only when a new candle starts.
func (p *StreamProcessor) HandleCandle(
	ctx context.Context,
	candle market.Candle,
) (*ProcessResult, error) {
	if candle.Market != p.market {
		return nil, fmt.Errorf(
			"market mismatch: expected=%s actual=%s",
			p.market,
			candle.Market,
		)
	}

	if candle.Time.IsZero() || candle.Open <= 0 {
		return nil, fmt.Errorf("invalid candle")
	}

	// Persist the latest candle update.
	if _, err := p.store.SaveCandles(
		ctx,
		p.unit,
		[]market.Candle{candle},
	); err != nil {
		return nil, fmt.Errorf("save realtime candle: %w", err)
	}

	// First message establishes the current candle.
	// Do not execute a signal at startup.
	if !p.initialized {
		p.currentCandleTime = candle.Time
		p.initialized = true
		return nil, nil
	}

	// Ignore older candle messages.
	if candle.Time.Before(p.currentCandleTime) {
		return nil, nil
	}

	// Same candle: only update its latest data.
	if candle.Time.Equal(p.currentCandleTime) {
		return nil, nil
	}

	expectedTime := p.currentCandleTime.Add(
		time.Duration(p.unit) * time.Minute,
	)

	// If a candle was missed, do not execute late.
	// Establish a new baseline and wait for the next boundary.
	if !candle.Time.Equal(expectedTime) {
		p.currentCandleTime = candle.Time
		return nil, nil
	}

	// Load completed candles from SQLite.
	history, err := p.store.GetCandles(
		ctx,
		p.market,
		p.unit,
		strategy.MinimumCandles+1,
	)
	if err != nil {
		return nil, fmt.Errorf("load candle history: %w", err)
	}

	// Exclude the currently forming execution candle.
	completed := make([]market.Candle, 0, len(history))

	for _, item := range history {
		if item.Time.Before(candle.Time) {
			completed = append(completed, item)
		}
	}

	// Not enough history yet.
	if len(completed) < strategy.MinimumCandles {
		p.currentCandleTime = candle.Time
		return nil, nil
	}

	// The newest candle is the execution candle.
	candles := append(completed, candle)

	result, err := p.engine.Process(ctx, candles)
	if err != nil {
		return nil, fmt.Errorf("process paper trade: %w", err)
	}

	p.currentCandleTime = candle.Time

	return &result, nil
}
