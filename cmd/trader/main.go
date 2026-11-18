package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/exchange"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/storage"
)

const (
	marketCode  = "KRW-BTC"
	candleUnit  = 5
	candleCount = 200
)

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Printf("Trader stopped with error: %v", err)
		os.Exit(1)
	}

	fmt.Println("Trader stopped.")
}

func run(ctx context.Context) error {
	fmt.Println("================================")
	fmt.Println("       Upbit Auto Trader")
	fmt.Println("================================")

	// 1. Open SQLite database.
	store, err := storage.OpenSQLite(ctx, "data/trader.db")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	// 2. Fetch historical candles through REST API.
	fmt.Println("\n[1] Fetching historical candles...")

	restCtx, cancel := context.WithTimeout(ctx, 15*time.Second)

	upbit := exchange.NewUpbitClient()
	candles, err := upbit.GetMinuteCandles(
		restCtx,
		marketCode,
		candleUnit,
		candleCount,
	)

	cancel()

	if err != nil {
		return fmt.Errorf("fetch historical candles: %w", err)
	}

	// 3. Save historical candles.
	savedCount, err := store.SaveCandles(ctx, candleUnit, candles)
	if err != nil {
		return fmt.Errorf("save historical candles: %w", err)
	}

	fmt.Printf("Historical candles saved or updated: %d\n", savedCount)

	// 4. Start WebSocket streaming.
	fmt.Println("\n[2] Starting WebSocket stream...")
	fmt.Printf("Market: %s\n", marketCode)
	fmt.Printf("Candle interval: %d minute(s)\n", candleUnit)
	fmt.Println("Press Ctrl+C to stop.")

	wsClient := exchange.NewUpbitWebSocketClient()

	err = wsClient.StreamMinuteCandles(
		ctx,
		marketCode,
		candleUnit,
		func(candle market.Candle) error {
			// Save or update the received candle.
			_, err := store.SaveCandles(
				ctx,
				candleUnit,
				[]market.Candle{candle},
			)
			if err != nil {
				return fmt.Errorf("save realtime candle: %w", err)
			}

			fmt.Printf(
				"[DB] %s | Close: %.0f | Volume: %.8f\n",
				candle.Time.Format("2006-01-02 15:04:05"),
				candle.Close,
				candle.Volume,
			)

			return nil
		},
	)

	// Ctrl+C is a normal shutdown.
	if ctx.Err() != nil {
		return nil
	}

	if err != nil {
		return fmt.Errorf("websocket stream: %w", err)
	}

	return nil
}
