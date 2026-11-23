package main

import (
	"context"
	"fmt"
	"os"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/exchange"
)

func main() {
	accessKey := os.Getenv("UPBIT_ACCESS_KEY")
	secretKey := os.Getenv("UPBIT_SECRET_KEY")

	client, err := exchange.NewUpbitPrivateClient(
		accessKey,
		secretKey,
	)
	if err != nil {
		fmt.Println("Client initialization failed:", err)
		os.Exit(1)
	}

	accounts, err := client.GetAccounts(context.Background())
	if err != nil {
		fmt.Println("Account inquiry failed:", err)
		os.Exit(1)
	}

	fmt.Println("========== UPBIT ACCOUNT ==========")

	for _, account := range accounts {
		if account.Currency != "KRW" &&
			account.Currency != "BTC" {
			continue
		}

		fmt.Printf("Currency       : %s\n", account.Currency)
		fmt.Printf("Balance        : %s\n", account.Balance)
		fmt.Printf("Locked         : %s\n", account.Locked)
		fmt.Printf("Average Buy    : %s\n", account.AvgBuyPrice)
		fmt.Println("-----------------------------------")
	}

	fmt.Println("Account inquiry completed.")
}
