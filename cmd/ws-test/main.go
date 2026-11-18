package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/exchange"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
)

func main() {
	fmt.Println("================================")
	fmt.Println("     Upbit WebSocket Test")
	fmt.Println("================================")

	// Handle Ctrl+C.
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	client := exchange.NewUpbitWebSocketClient()

	err := client.StreamMinuteCandles(
		ctx,
		"KRW-BTC",
		5,
		func(candle market.Candle) error {
			// Additional processing will be implemented later.
			return nil
		},
	)

	if err != nil {
		log.Fatalf("WebSocket error: %v", err)
	}

	fmt.Println("WebSocket stopped")
}
