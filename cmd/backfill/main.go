package main

import (
	"context"
	"flag"
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
	marketCode = "KRW-BTC"
	candleUnit = 5
	pageSize   = 200
)

func main() {
	days := flag.Int("days", 30, "number of historical days to fetch")
	flag.Parse()

	if *days <= 0 {
		log.Fatal("days must be greater than zero")
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	if err := run(ctx, *days); err != nil {
		log.Fatalf("Backfill failed: %v", err)
	}
}

func run(ctx context.Context, days int) error {
	fmt.Println("================================")
	fmt.Println("       Historical Backfill")
	fmt.Println("================================")
	fmt.Printf("Market: %s\n", marketCode)
	fmt.Printf("Interval: %d minutes\n", candleUnit)
	fmt.Printf("Period: %d days\n", days)

	store, err := storage.OpenSQLite(ctx, "data/trader.db")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	client := exchange.NewUpbitClient()

	startTime := time.Now().UTC().AddDate(0, 0, -days)
	cursor := time.Now().UTC()

	var totalSaved int64
	page := 0

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)

		candles, err := client.GetMinuteCandlesBefore(
			requestCtx,
			marketCode,
			candleUnit,
			pageSize,
			cursor,
		)

		cancel()

		if err != nil {
			return fmt.Errorf("fetch historical candles: %w", err)
		}

		if len(candles) == 0 {
			fmt.Println("[BACKFILL] No more candles.")
			break
		}

		oldestTime := candles[0].Time

		// Ensure that pagination always moves backward.
		if !oldestTime.Before(cursor) {
			return fmt.Errorf(
				"pagination did not move backward: cursor=%s oldest=%s",
				cursor.Format(time.RFC3339),
				oldestTime.Format(time.RFC3339),
			)
		}

		// Keep only candles inside the requested period.
		filtered := make([]market.Candle, 0, len(candles))

		for _, candle := range candles {
			if !candle.Time.Before(startTime) {
				filtered = append(filtered, candle)
			}
		}

		if len(filtered) > 0 {
			saved, err := store.SaveCandles(
				ctx,
				candleUnit,
				filtered,
			)
			if err != nil {
				return fmt.Errorf("save historical candles: %w", err)
			}

			totalSaved += saved
		}

		page++

		fmt.Printf(
			"[BACKFILL] Page: %d | Fetched: %d | Saved/Updated: %d | Oldest: %s\n",
			page,
			len(candles),
			len(filtered),
			oldestTime.Format("2006-01-02 15:04:05"),
		)

		// The requested start time has been reached.
		if !oldestTime.After(startTime) {
			break
		}

		// Upbit's 'to' parameter is exclusive.
		// Use the oldest candle time as the next cursor.
		cursor = oldestTime

		// Keep API requests comfortably spaced apart.
		if !wait(ctx, 200*time.Millisecond) {
			return ctx.Err()
		}
	}

	fmt.Println("--------------------------------")
	fmt.Printf("Backfill completed. Total saved/updated: %d\n", totalSaved)
	fmt.Println("--------------------------------")

	return nil
}

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
