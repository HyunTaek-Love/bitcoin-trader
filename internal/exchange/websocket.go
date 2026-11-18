package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
	"github.com/gorilla/websocket"
)

const websocketURL = "wss://api.upbit.com/websocket/v1"

// UpbitWebSocketClient handles Upbit WebSocket connections.
type UpbitWebSocketClient struct {
	dialer *websocket.Dialer
}

// NewUpbitWebSocketClient creates a WebSocket client.
func NewUpbitWebSocketClient() *UpbitWebSocketClient {
	return &UpbitWebSocketClient{
		dialer: &websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

type candleSubscription struct {
	Type  string   `json:"type"`
	Codes []string `json:"codes"`
}

type candleWebSocketResponse struct {
	Type          string  `json:"type"`
	Code          string  `json:"code"`
	CandleTimeUTC string  `json:"candle_date_time_utc"`
	OpeningPrice  float64 `json:"opening_price"`
	HighPrice     float64 `json:"high_price"`
	LowPrice      float64 `json:"low_price"`
	TradePrice    float64 `json:"trade_price"`
	TradeVolume   float64 `json:"candle_acc_trade_volume"`
	TradeAmount   float64 `json:"candle_acc_trade_price"`
	StreamType    string  `json:"stream_type"`
}

type websocketErrorResponse struct {
	Error *struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"error"`
}

// StreamMinuteCandles subscribes to Upbit minute candle updates.
func (c *UpbitWebSocketClient) StreamMinuteCandles(
	ctx context.Context,
	marketCode string,
	unit int,
	onCandle func(market.Candle) error,
) error {

	validUnits := map[int]bool{
		1:   true,
		3:   true,
		5:   true,
		10:  true,
		15:  true,
		30:  true,
		60:  true,
		240: true,
	}

	if !validUnits[unit] {
		return fmt.Errorf("invalid candle unit: %d", unit)
	}

	if marketCode == "" {
		return fmt.Errorf("market code must not be empty")
	}

	if onCandle == nil {
		return fmt.Errorf("onCandle callback must not be nil")
	}

	// Connect to Upbit WebSocket.
	conn, _, err := c.dialer.DialContext(
		ctx,
		websocketURL,
		http.Header{},
	)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}
	defer conn.Close()

	fmt.Println("WebSocket connected")

	// Close the connection when the context is cancelled.
	stopClose := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stopClose()

	conn.SetReadLimit(1024 * 1024)

	// Set initial read deadline.
	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))

	// Extend read deadline when Pong is received.
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(
			time.Now().Add(70 * time.Second),
		)
	})

	// Subscribe to candle updates.
	request := []any{
		map[string]string{
			"ticket": fmt.Sprintf("bitcoin-trader-%d", time.Now().UnixNano()),
		},
		candleSubscription{
			Type:  fmt.Sprintf("candle.%dm", unit),
			Codes: []string{marketCode},
		},
		map[string]string{
			"format": "DEFAULT",
		},
	}

	if err := conn.WriteJSON(request); err != nil {
		return fmt.Errorf("subscription request failed: %w", err)
	}

	fmt.Printf(
		"Subscribed: %s %dm candles\n",
		marketCode,
		unit,
	)

	// Send periodic Ping frames.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				err := conn.WriteControl(
					websocket.PingMessage,
					[]byte("ping"),
					time.Now().Add(5*time.Second),
				)

				if err != nil {
					_ = conn.Close()
					return
				}
			}
		}
	}()

	kst, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	// Receive WebSocket messages.
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return fmt.Errorf("websocket read failed: %w", err)
		}

		// Check API error response.
		var apiError websocketErrorResponse

		if err := json.Unmarshal(message, &apiError); err != nil {
			return fmt.Errorf("decode websocket response: %w", err)
		}

		if apiError.Error != nil {
			return fmt.Errorf(
				"upbit websocket error: %s - %s",
				apiError.Error.Name,
				apiError.Error.Message,
			)
		}

		// Decode candle data.
		var response candleWebSocketResponse

		if err := json.Unmarshal(message, &response); err != nil {
			return fmt.Errorf("decode candle: %w", err)
		}

		if response.Code == "" || response.CandleTimeUTC == "" {
			return fmt.Errorf("unexpected websocket message")
		}

		candleTime, err := time.Parse(
			"2006-01-02T15:04:05",
			response.CandleTimeUTC,
		)
		if err != nil {
			return fmt.Errorf("parse candle time: %w", err)
		}

		candle := market.Candle{
			Market:      response.Code,
			Time:        candleTime.UTC(),
			Open:        response.OpeningPrice,
			High:        response.HighPrice,
			Low:         response.LowPrice,
			Close:       response.TradePrice,
			Volume:      response.TradeVolume,
			TradeAmount: response.TradeAmount,
		}

		if err := onCandle(candle); err != nil {
			return fmt.Errorf("candle callback failed: %w", err)
		}

		fmt.Printf(
			"[%s] %s Close: %.0f | Volume: %.8f | Stream: %s\n",
			candle.Time.In(kst).Format("2006-01-02 15:04:05"),
			candle.Market,
			candle.Close,
			candle.Volume,
			response.StreamType,
		)
	}
}
