package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/backtest"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/storage"
)

func main() {
	dbPath := flag.String(
		"db",
		"data/trader.db",
		"SQLite database path",
	)

	marketCode := flag.String(
		"market",
		"KRW-BTC",
		"Market code",
	)

	unit := flag.Int(
		"unit",
		5,
		"Candle interval in minutes",
	)

	limit := flag.Int(
		"limit",
		100000,
		"Maximum number of candles to load",
	)

	initialCapital := flag.Float64(
		"capital",
		100000,
		"Initial capital in KRW",
	)

	feeRate := flag.Float64(
		"fee",
		0.0005,
		"Trading fee rate (0.0005 = 0.05%)",
	)

	flag.Parse()

	ctx := context.Background()

	// 1. Open SQLite database.
	store, err := storage.OpenSQLite(ctx, *dbPath)
	if err != nil {
		log.Fatalf("Open database: %v", err)
	}
	defer store.Close()

	// 2. Load historical candles.
	candles, err := store.GetCandles(
		ctx,
		*marketCode,
		*unit,
		*limit,
	)
	if err != nil {
		log.Fatalf("Load candles: %v", err)
	}

	if len(candles) == 0 {
		log.Fatal("No candles found in database")
	}

	fmt.Println("================================")
	fmt.Println("         Backtest Runner")
	fmt.Println("================================")
	fmt.Printf("Market:          %s\n", *marketCode)
	fmt.Printf("Candle interval: %d minutes\n", *unit)
	fmt.Printf("Loaded candles:  %d\n", len(candles))
	fmt.Printf("Period:          %s ~ %s\n",
		candles[0].Time.Format("2006-01-02 15:04"),
		candles[len(candles)-1].Time.Format("2006-01-02 15:04"),
	)

	// 3. Run backtest.
	result, err := backtest.Run(
		candles,
		backtest.Config{
			InitialCapital: *initialCapital,
			FeeRate:        *feeRate,
		},
	)
	if err != nil {
		log.Fatalf("Run backtest: %v", err)
	}

	// 4. Print summary.
	fmt.Println("\n========== Backtest Result ==========")
	fmt.Printf("Initial Capital:  %.0f KRW\n", result.InitialCapital)
	fmt.Printf("Final Capital:    %.0f KRW\n", result.FinalCapital)
	fmt.Printf("Total Return:     %.2f%%\n", result.TotalReturnPct)
	fmt.Printf("Total Trades:     %d\n", result.TotalTrades)
	fmt.Printf("Winning Trades:   %d\n", result.WinningTrades)
	fmt.Printf("Win Rate:         %.2f%%\n", result.WinRatePct)
	fmt.Printf("Max Drawdown:     %.2f%%\n", result.MaxDrawdownPct)
	fmt.Printf("Buy & Hold:       %.0f KRW\n", result.BuyHoldFinalCapital)
	fmt.Printf("Buy & Hold Return: %.2f%%\n", result.BuyHoldReturnPct)

	fmt.Printf("Average Win:      %+.0f KRW\n", result.AverageWinKRW)
	fmt.Printf("Average Loss:     %+.0f KRW\n", result.AverageLossKRW)
	fmt.Printf("Profit Factor:    %.2f\n", result.ProfitFactor)

	fmt.Printf("Max Consecutive Losses: %d\n",
		result.MaxConsecutiveLosses)

	// 5. Print individual trades.
	fmt.Println("\n============== Trades ==============")

	if len(result.Trades) == 0 {
		fmt.Println("No trades executed.")
		return
	}

	for i, trade := range result.Trades {
		fmt.Printf(
			"#%d | BUY: %s @ %.0f | SELL: %s @ %.0f | Profit: %+.0f KRW (%+.2f%%)\n",
			i+1,
			trade.EntryTime,
			trade.EntryPrice,
			trade.ExitTime,
			trade.ExitPrice,
			trade.Profit,
			trade.ProfitPct,
		)
	}

	fmt.Println("=====================================")
}
