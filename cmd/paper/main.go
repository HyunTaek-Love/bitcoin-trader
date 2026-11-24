package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/exchange"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/paper"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/storage"
)

const (
	defaultMarket = "KRW-BTC"
	defaultUnit   = 5
	candleCount   = 200
)

func main() {
	accountID := flag.String(
		"id",
		"paper-krw-btc-5m",
		"paper account ID",
	)

	initialCash := flag.Float64(
		"capital",
		100000,
		"initial capital in KRW",
	)

	runMode := flag.Bool(
		"run",
		false,
		"start realtime paper trading",
	)

	flag.Parse()

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	if err := run(ctx, *accountID, *initialCash, *runMode); err != nil {
		log.Printf("Paper trader stopped with error: %v", err)
		os.Exit(1)
	}

	fmt.Println("Paper trader stopped.")
}

func run(
	ctx context.Context,
	accountID string,
	initialCash float64,
	runMode bool,
) error {
	store, err := storage.OpenSQLite(ctx, "data/trader.db")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	account, err := store.GetPaperAccount(ctx, accountID)

	if err != nil {
		if !errors.Is(err, storage.ErrPaperAccountNotFound) {
			return fmt.Errorf("get paper account: %w", err)
		}

		if err := store.CreatePaperAccount(
			ctx,
			storage.PaperAccount{
				AccountID:   accountID,
				Market:      defaultMarket,
				Unit:        defaultUnit,
				InitialCash: initialCash,
			},
		); err != nil {
			return fmt.Errorf("create paper account: %w", err)
		}

		fmt.Println("Paper account created.")
	} else {
		fmt.Println("Existing paper account loaded.")
	}

	account, err = store.GetPaperAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("reload paper account: %w", err)
	}

	printAccount(account)

	if !runMode {
		fmt.Println("Account information only. Use -run to start paper trading.")
		return nil
	}

	return runPaperTrading(ctx, store, account)
}

func runPaperTrading(
	ctx context.Context,
	store *storage.SQLiteStore,
	account *storage.PaperAccount,
) error {
	fmt.Println()
	fmt.Println("================================")
	fmt.Println("       REALTIME PAPER TRADING")
	fmt.Println("================================")
	fmt.Printf("Market    : %s\n", account.Market)
	fmt.Printf("Interval  : %d minutes\n", account.Unit)
	fmt.Printf("Fee Rate  : %.4f%%\n", paper.DefaultFeeRate*100)
	fmt.Println("Real orders: DISABLED")
	fmt.Println("================================")

	engine, err := paper.NewEngine(
		store,
		paper.Config{
			AccountID: account.AccountID,
			FeeRate:   paper.DefaultFeeRate,
		},
	)
	if err != nil {
		return fmt.Errorf("create paper engine: %w", err)
	}

	processor, err := paper.NewStreamProcessor(
		store,
		engine,
		account.Market,
		account.Unit,
	)
	if err != nil {
		return fmt.Errorf("create stream processor: %w", err)
	}

	restClient := exchange.NewUpbitClient()
	wsClient := exchange.NewUpbitWebSocketClient()

	retryDelay := 5 * time.Second

	for {
		if ctx.Err() != nil {
			return nil
		}

		// Synchronize historical candles before connecting.
		if err := syncHistoricalCandles(
			ctx,
			restClient,
			store,
			account.Market,
			account.Unit,
		); err != nil {
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

		fmt.Println("[WEBSOCKET] Connecting...")

		receivedMessage := false

		err := wsClient.StreamMinuteCandles(
			ctx,
			account.Market,
			account.Unit,
			func(candle market.Candle) error {
				receivedMessage = true

				result, err := processor.HandleCandle(
					ctx,
					candle,
				)
				if err != nil {
					return err
				}

				if result != nil {
					log.Printf(
						"[SIGNAL] Action=%s | Reason=%s",
						result.Action,
						result.Reason,
					)

					if result.Executed && result.Trade != nil {
						trade := result.Trade

						log.Printf(
							"[PAPER TRADE] %s | Price=%.0f | Volume=%.8f | Fee=%.2f | Total=%.2f",
							trade.Side,
							trade.Price,
							trade.Volume,
							trade.Fee,
							trade.Total,
						)

						printCurrentAccount(
							ctx,
							store,
							account.AccountID,
						)
					}
				}

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

func syncHistoricalCandles(
	ctx context.Context,
	client *exchange.UpbitClient,
	store *storage.SQLiteStore,
	marketCode string,
	unit int,
) error {
	fmt.Println("[REST] Synchronizing historical candles...")

	restCtx, cancel := context.WithTimeout(
		ctx,
		15*time.Second,
	)
	defer cancel()

	candles, err := client.GetMinuteCandles(
		restCtx,
		marketCode,
		unit,
		candleCount,
	)
	if err != nil {
		return fmt.Errorf("fetch historical candles: %w", err)
	}

	savedCount, err := store.SaveCandles(
		ctx,
		unit,
		candles,
	)
	if err != nil {
		return fmt.Errorf("save historical candles: %w", err)
	}

	fmt.Printf(
		"[REST] Synchronized %d candles\n",
		savedCount,
	)

	return nil
}

func printAccount(account *storage.PaperAccount) {
	fmt.Println("================================")
	fmt.Println("        PAPER ACCOUNT")
	fmt.Println("================================")
	fmt.Printf("Account ID : %s\n", account.AccountID)
	fmt.Printf("Market     : %s\n", account.Market)
	fmt.Printf("Interval   : %d minutes\n", account.Unit)
	fmt.Printf("Initial    : %.0f KRW\n", account.InitialCash)
	fmt.Printf("Cash       : %.0f KRW\n", account.Cash)
	fmt.Printf("BTC        : %.8f BTC\n", account.AssetVolume)
	fmt.Printf("Avg Price  : %.0f KRW\n", account.AvgBuyPrice)
	fmt.Printf("Realized   : %.0f KRW\n", account.RealizedPnL)
	fmt.Println("================================")
}

func printCurrentAccount(
	ctx context.Context,
	store *storage.SQLiteStore,
	accountID string,
) {
	account, err := store.GetPaperAccount(ctx, accountID)
	if err != nil {
		log.Printf("[ACCOUNT] Failed to load account: %v", err)
		return
	}

	fmt.Printf(
		"[ACCOUNT] Cash=%.0f KRW | BTC=%.8f | Realized PnL=%.0f KRW\n",
		account.Cash,
		account.AssetVolume,
		account.RealizedPnL,
	)
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

func nextRetryDelay(current time.Duration) time.Duration {
	next := current * 2

	if next > 60*time.Second {
		next = 60 * time.Second
	}

	return next
}
