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

	store, err := storage.OpenSQLite(ctx, "data/trader.db")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	restClient := exchange.NewUpbitClient()
	wsClient := exchange.NewUpbitWebSocketClient()

	retryDelay := 5 * time.Second

	for {
		if ctx.Err() != nil {
			return nil
		}

		// 1. Restore historical candles before connecting.
		if err := syncHistoricalCandles(ctx, restClient, store); err != nil {
			if ctx.Err() != nil {
				return nil
			}

			log.Printf("[RECOVERY] Historical sync failed: %v", err)
			log.Printf("[RETRY] Retrying in %v", retryDelay)

			if !wait(ctx, retryDelay) {
				return nil
			}

			retryDelay = nextRetryDelay(retryDelay)
			continue
		}

		// 2. Start WebSocket stream.
		fmt.Println("\n[WEBSOCKET] Connecting...")
		fmt.Printf("Market: %s | Interval: %d minute(s)\n",
			marketCode, candleUnit)

		receivedMessage := false

		err := wsClient.StreamMinuteCandles(
			ctx,
			marketCode,
			candleUnit,
			func(candle market.Candle) error {
				receivedMessage = true

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

		if ctx.Err() != nil {
			return nil
		}

		if err == nil {
			err = fmt.Errorf("websocket stopped unexpectedly")
		}

		log.Printf("[WEBSOCKET] Connection lost: %v", err)

		// If messages were received, the connection worked.
		// Reset the retry delay for the next disconnection.
		if receivedMessage {
			retryDelay = 5 * time.Second
		}

		log.Printf("[RETRY] Reconnecting in %v", retryDelay)

		if !wait(ctx, retryDelay) {
			return nil
		}

		retryDelay = nextRetryDelay(retryDelay)
	}
}

// syncHistoricalCandles fetches recent candles from REST API
// and saves them to SQLite to recover any missing data.
func syncHistoricalCandles(
	ctx context.Context,
	client *exchange.UpbitClient,
	store *storage.SQLiteStore,
) error {
	fmt.Println("\n[REST] Synchronizing historical candles...")

	restCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	candles, err := client.GetMinuteCandles(
		restCtx,
		marketCode,
		candleUnit,
		candleCount,
	)
	if err != nil {
		return fmt.Errorf("fetch historical candles: %w", err)
	}

	savedCount, err := store.SaveCandles(
		ctx,
		candleUnit,
		candles,
	)
	if err != nil {
		return fmt.Errorf("save historical candles: %w", err)
	}

	fmt.Printf(
		"[REST] Synchronized %d candles (saved or updated)\n",
		savedCount,
	)

	return nil
}

// wait waits for the specified duration.
// It returns false when the context is canceled.
func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// nextRetryDelay doubles the retry delay up to 60 seconds.
func nextRetryDelay(current time.Duration) time.Duration {
	next := current * 2

	if next > 60*time.Second {
		next = 60 * time.Second
	}

	return next
}
