package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/exchange"
	"github.com/HyunTaek-Love/bitcoin-trader/internal/indicator"
)

func main() {
	fmt.Println("================================")
	fmt.Println("       Upbit Auto Trader")
	fmt.Println("================================")

	client := exchange.NewUpbitClient()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	candles, err := client.GetMinuteCandles(
		ctx,
		"KRW-BTC",
		5,
		200,
	)

	if err != nil {
		log.Fatalf("Failed to get candles: %v", err)
	}

	fmt.Printf("\nRetrieved candles: %d\n", len(candles))

	fmt.Println("\nLatest 10 candles")
	fmt.Println(
		"---------------------------------------------------------------",
	)
	fmt.Printf(
		"%-20s %14s %14s %14s %14s\n",
		"TIME(KST)",
		"OPEN",
		"HIGH",
		"LOW",
		"CLOSE",
	)

	kst, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		log.Fatalf("Failed to load timezone: %v", err)
	}

	start := len(candles) - 10
	if start < 0 {
		start = 0
	}

	for _, candle := range candles[start:] {
		fmt.Printf(
			"%-20s %14.0f %14.0f %14.0f %14.0f\n",
			candle.Time.In(kst).Format("2006-01-02 15:04"),
			candle.Open,
			candle.High,
			candle.Low,
			candle.Close,
		)
	}

	fmt.Println(
		"---------------------------------------------------------------",
	)

	if len(candles) > 0 {
		latest := candles[len(candles)-1]

		fmt.Printf(
			"\nLatest BTC price: %.0f KRW\n",
			latest.Close,
		)

		fmt.Printf(
			"Latest candle time: %s\n",
			latest.Time.In(kst).Format(time.RFC3339),
		)
	}

	// Extract closing prices.
	prices := make([]float64, 0, len(candles))

	for _, candle := range candles {
		prices = append(prices, candle.Close)
	}

	// Calculate technical indicators.
	result, err := indicator.Calculate(prices)

	if err != nil {
		log.Fatalf("Failed to calculate indicators: %v", err)
	}

	fmt.Println("\nTechnical Indicators")
	fmt.Println("--------------------------------")

	fmt.Printf("SMA20 : %.2f KRW\n", result.SMA20)
	fmt.Printf("SMA50 : %.2f KRW\n", result.SMA50)
	fmt.Printf("RSI14 : %.2f\n", result.RSI14)

	fmt.Println("--------------------------------")
}
