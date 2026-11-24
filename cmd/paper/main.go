package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/storage"
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

	flag.Parse()

	ctx := context.Background()

	store, err := storage.OpenSQLite(ctx, "data/trader.db")
	if err != nil {
		log.Fatalf("Open database failed: %v", err)
	}
	defer store.Close()

	account, err := store.GetPaperAccount(ctx, *accountID)

	if err != nil {
		if !errors.Is(err, storage.ErrPaperAccountNotFound) {
			log.Fatalf("Get paper account failed: %v", err)
		}

		err = store.CreatePaperAccount(ctx, storage.PaperAccount{
			AccountID:   *accountID,
			Market:      "KRW-BTC",
			Unit:        5,
			InitialCash: *initialCash,
		})
		if err != nil {
			log.Fatalf("Create paper account failed: %v", err)
		}

		fmt.Println("Paper account created.")
	} else {
		fmt.Println("Existing paper account loaded.")
	}

	account, err = store.GetPaperAccount(ctx, *accountID)
	if err != nil {
		log.Fatalf("Get paper account failed: %v", err)
	}

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
