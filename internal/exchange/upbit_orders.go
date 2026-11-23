package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Order represents an Upbit order.
type Order struct {
	UUID            string `json:"uuid"`
	Market          string `json:"market"`
	Side            string `json:"side"`
	OrdType         string `json:"ord_type"`
	Price           string `json:"price"`
	State           string `json:"state"`
	Volume          string `json:"volume"`
	RemainingVolume string `json:"remaining_volume"`
	ExecutedVolume  string `json:"executed_volume"`
	PaidFee         string `json:"paid_fee"`
	CreatedAt       string `json:"created_at"`
	TradesCount     int    `json:"trades_count"`
}

func validatePositiveNumber(name, value string) error {
	value = strings.TrimSpace(value)

	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number <= 0 || math.IsNaN(number) || math.IsInf(number, 0) {
		return fmt.Errorf("%s must be a valid positive number", name)
	}

	return nil
}

// doPrivateRequest sends an authenticated private API request.
func (c *UpbitPrivateClient) doPrivateRequest(
	ctx context.Context,
	method string,
	endpoint string,
	queryString string,
	body []byte,
) (*http.Response, error) {
	token, err := c.createJWT(queryString)
	if err != nil {
		return nil, err
	}

	requestURL := privateBaseURL + endpoint
	if method == http.MethodGet || method == http.MethodDelete {
		if queryString != "" {
			requestURL += "?" + queryString
		}
	}

	var requestBody io.Reader
	if body != nil {
		requestBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		requestURL,
		requestBody,
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send private request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()

		responseBody, _ := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)

		return nil, fmt.Errorf(
			"upbit API error: status=%d body=%s",
			resp.StatusCode,
			string(responseBody),
		)
	}

	return resp, nil
}

// CreateMarketBuyOrder creates a market buy order.
// amount is the total KRW amount to spend.
func (c *UpbitPrivateClient) CreateMarketBuyOrder(
	ctx context.Context,
	market string,
	amount string,
) (*Order, error) {
	if strings.TrimSpace(market) == "" {
		return nil, fmt.Errorf("market must not be empty")
	}

	if err := validatePositiveNumber("amount", amount); err != nil {
		return nil, err
	}

	params := map[string]string{
		"market":   market,
		"side":     "bid",
		"price":    amount,
		"ord_type": "price",
	}

	return c.createOrder(ctx, params)
}

// CreateMarketSellOrder creates a market sell order.
// volume is the amount of cryptocurrency to sell.
func (c *UpbitPrivateClient) CreateMarketSellOrder(
	ctx context.Context,
	market string,
	volume string,
) (*Order, error) {
	if strings.TrimSpace(market) == "" {
		return nil, fmt.Errorf("market must not be empty")
	}

	if err := validatePositiveNumber("volume", volume); err != nil {
		return nil, err
	}

	params := map[string]string{
		"market":   market,
		"side":     "ask",
		"volume":   volume,
		"ord_type": "market",
	}

	return c.createOrder(ctx, params)
}

func (c *UpbitPrivateClient) createOrder(
	ctx context.Context,
	params map[string]string,
) (*Order, error) {
	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}

	queryString := query.Encode()

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode order body: %w", err)
	}

	resp, err := c.doPrivateRequest(
		ctx,
		http.MethodPost,
		"/orders",
		queryString,
		body,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, fmt.Errorf("decode order response: %w", err)
	}

	return &order, nil
}

// GetOrder retrieves an order by UUID.
func (c *UpbitPrivateClient) GetOrder(
	ctx context.Context,
	orderUUID string,
) (*Order, error) {
	if strings.TrimSpace(orderUUID) == "" {
		return nil, fmt.Errorf("order UUID must not be empty")
	}

	query := url.Values{}
	query.Set("uuid", orderUUID)
	queryString := query.Encode()

	resp, err := c.doPrivateRequest(
		ctx,
		http.MethodGet,
		"/order",
		queryString,
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, fmt.Errorf("decode order response: %w", err)
	}

	return &order, nil
}

// CancelOrder cancels an order by UUID.
func (c *UpbitPrivateClient) CancelOrder(
	ctx context.Context,
	orderUUID string,
) (*Order, error) {
	if strings.TrimSpace(orderUUID) == "" {
		return nil, fmt.Errorf("order UUID must not be empty")
	}

	query := url.Values{}
	query.Set("uuid", orderUUID)
	queryString := query.Encode()

	resp, err := c.doPrivateRequest(
		ctx,
		http.MethodDelete,
		"/order",
		queryString,
		nil,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, fmt.Errorf("decode order response: %w", err)
	}

	return &order, nil
}
