package exchange

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetMinuteCandlesBefore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/candles/minutes/5" {
				t.Errorf(
					"unexpected path: %s",
					r.URL.Path,
				)
			}

			query := r.URL.Query()

			if query.Get("market") != "KRW-BTC" {
				t.Errorf("unexpected market: %s", query.Get("market"))
			}

			if query.Get("count") != "2" {
				t.Errorf("unexpected count: %s", query.Get("count"))
			}

			expectedTo := "2026-01-01T00:10:00Z"

			if query.Get("to") != expectedTo {
				t.Errorf(
					"unexpected to parameter: expected=%s actual=%s",
					expectedTo,
					query.Get("to"),
				)
			}

			w.Header().Set("Content-Type", "application/json")

			fmt.Fprint(w, `[
				{
					"market": "KRW-BTC",
					"candle_date_time_utc": "2026-01-01T00:05:00",
					"opening_price": 101000000,
					"high_price": 102000000,
					"low_price": 100000000,
					"trade_price": 101500000,
					"candle_acc_trade_volume": 1.5,
					"candle_acc_trade_price": 152250000
				},
				{
					"market": "KRW-BTC",
					"candle_date_time_utc": "2026-01-01T00:00:00",
					"opening_price": 100000000,
					"high_price": 101000000,
					"low_price": 99000000,
					"trade_price": 100500000,
					"candle_acc_trade_volume": 2.0,
					"candle_acc_trade_price": 201000000
				}
			]`)
		},
	))
	defer server.Close()

	client := NewUpbitClient()
	client.httpClient = server.Client()
	client.baseURL = server.URL

	to := time.Date(
		2026, 1, 1, 9, 10, 0, 0,
		time.FixedZone("KST", 9*60*60),
	)

	candles, err := client.GetMinuteCandlesBefore(
		context.Background(),
		"KRW-BTC",
		5,
		2,
		to,
	)
	if err != nil {
		t.Fatalf("GetMinuteCandlesBefore failed: %v", err)
	}

	if len(candles) != 2 {
		t.Fatalf(
			"expected 2 candles, got %d",
			len(candles),
		)
	}

	expectedTimes := []time.Time{
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC),
	}

	for i, candle := range candles {
		if !candle.Time.Equal(expectedTimes[i]) {
			t.Errorf(
				"unexpected candle time at index %d: expected=%s actual=%s",
				i,
				expectedTimes[i],
				candle.Time,
			)
		}

		if candle.Market != "KRW-BTC" {
			t.Errorf(
				"unexpected market at index %d: %s",
				i,
				candle.Market,
			)
		}
	}
}

func TestGetMinuteCandlesBeforeRejectsZeroTime(t *testing.T) {
	client := NewUpbitClient()

	_, err := client.GetMinuteCandlesBefore(
		context.Background(),
		"KRW-BTC",
		5,
		10,
		time.Time{},
	)

	if err == nil {
		t.Fatal("expected error for zero time")
	}
}
