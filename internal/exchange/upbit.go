package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/HyunTaek-Love/bitcoin-trader/internal/market"
)

const baseURL = "https://api.upbit.com/v1"

// UpbitClient handles public Upbit API requests.
type UpbitClient struct {
	httpClient *http.Client
}

// NewUpbitClient creates an Upbit API client.
func NewUpbitClient() *UpbitClient {
	return &UpbitClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// API response structure.
type candleResponse struct {
	Market         string  `json:"market"`
	CandleDateTime string  `json:"candle_date_time_utc"`
	OpeningPrice   float64 `json:"opening_price"`
	HighPrice      float64 `json:"high_price"`
	LowPrice       float64 `json:"low_price"`
	TradePrice     float64 `json:"trade_price"`
	Timestamp      int64   `json:"timestamp"`
	TradeVolume    float64 `json:"candle_acc_trade_volume"`
	TradeAmount    float64 `json:"candle_acc_trade_price"`
}

// GetMinuteCandles retrieves the latest minute candles.
func (c *UpbitClient) GetMinuteCandles(
	ctx context.Context,
	marketCode string,
	unit int,
	count int,
) ([]market.Candle, error) {
	return c.getMinuteCandles(
		ctx,
		marketCode,
		unit,
		count,
		time.Time{},
	)
}

// GetMinuteCandlesBefore retrieves candles before the specified time.
func (c *UpbitClient) GetMinuteCandlesBefore(
	ctx context.Context,
	marketCode string,
	unit int,
	count int,
	to time.Time,
) ([]market.Candle, error) {
	if to.IsZero() {
		return nil, fmt.Errorf("to time must not be zero")
	}

	return c.getMinuteCandles(
		ctx,
		marketCode,
		unit,
		count,
		to,
	)
}

// getMinuteCandles retrieves candles with an optional end time.
func (c *UpbitClient) getMinuteCandles(
	ctx context.Context,
	marketCode string,
	unit int,
	count int,
	to time.Time,
) ([]market.Candle, error) {

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
		return nil, fmt.Errorf("invalid candle unit: %d", unit)
	}

	if count < 1 || count > 200 {
		return nil, fmt.Errorf("count must be between 1 and 200")
	}

	if marketCode == "" {
		return nil, fmt.Errorf("market code must not be empty")
	}

	endpoint := fmt.Sprintf(
		"%s/candles/minutes/%d",
		baseURL,
		unit,
	)

	params := url.Values{}
	params.Set("market", marketCode)
	params.Set("count", strconv.Itoa(count))

	// Add the 'to' parameter when historical data is requested.
	if !to.IsZero() {
		params.Set("to", to.UTC().Format(time.RFC3339))
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint+"?"+params.Encode(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request candles: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)

		return nil, fmt.Errorf(
			"upbit API error: status=%d body=%s",
			resp.StatusCode,
			string(body),
		)
	}

	var response []candleResponse

	decoder := json.NewDecoder(
		io.LimitReader(resp.Body, 2*1024*1024),
	)

	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	candles := make([]market.Candle, 0, len(response))

	for _, item := range response {
		candleTime, err := time.Parse(
			"2006-01-02T15:04:05",
			item.CandleDateTime,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"parse candle time %q: %w",
				item.CandleDateTime,
				err,
			)
		}

		candles = append(candles, market.Candle{
			Market:      item.Market,
			Time:        candleTime.UTC(),
			Open:        item.OpeningPrice,
			High:        item.HighPrice,
			Low:         item.LowPrice,
			Close:       item.TradePrice,
			Volume:      item.TradeVolume,
			TradeAmount: item.TradeAmount,
		})
	}

	// Sort candles from oldest to newest.
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Time.Before(candles[j].Time)
	})

	// Remove incomplete candles.
	now := time.Now().UTC()

	closedCandles := make([]market.Candle, 0, len(candles))

	for _, candle := range candles {
		if candle.IsClosed(unit, now) {
			closedCandles = append(closedCandles, candle)
		}
	}

	return closedCandles, nil
}
