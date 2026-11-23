package exchange

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testAccessKey = "test-access-key"
	testSecretKey = "test-secret-key"
)

// roundTripFunc allows tests to intercept HTTP requests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newTestClient creates a client that never accesses the real Upbit API.
func newTestClient(
	t *testing.T,
	handler func(*http.Request) (*http.Response, error),
) *UpbitPrivateClient {
	t.Helper()

	client, err := NewUpbitPrivateClient(
		testAccessKey,
		testSecretKey,
	)
	if err != nil {
		t.Fatal(err)
	}

	client.httpClient = &http.Client{
		Transport: roundTripFunc(handler),
	}

	return client
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// verifyJWT checks the token signature and query hash.
func verifyJWT(
	t *testing.T,
	req *http.Request,
	expectedQuery string,
) {
	t.Helper()

	authHeader := req.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		t.Fatal("Authorization header is missing or invalid")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	token, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS512 {
				return nil, fmt.Errorf(
					"unexpected signing method: %v",
					token.Method.Alg(),
				)
			}

			return []byte(testSecretKey), nil
		},
	)
	if err != nil {
		t.Fatalf("JWT verification failed: %v", err)
	}

	if !token.Valid {
		t.Fatal("JWT is invalid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("JWT claims have invalid type")
	}

	if claims["access_key"] != testAccessKey {
		t.Errorf("unexpected access_key: %v", claims["access_key"])
	}

	nonce, ok := claims["nonce"].(string)
	if !ok || nonce == "" {
		t.Error("JWT nonce is missing")
	}

	if expectedQuery == "" {
		if _, exists := claims["query_hash"]; exists {
			t.Error("query_hash should not exist")
		}
		return
	}

	hash := sha512.Sum512([]byte(expectedQuery))
	expectedHash := hex.EncodeToString(hash[:])

	if claims["query_hash"] != expectedHash {
		t.Errorf(
			"query_hash mismatch: got=%v expected=%s",
			claims["query_hash"],
			expectedHash,
		)
	}

	if claims["query_hash_alg"] != "SHA512" {
		t.Errorf(
			"unexpected query_hash_alg: %v",
			claims["query_hash_alg"],
		)
	}
}

func TestCreateMarketBuyOrder(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", req.Method)
		}

		if req.URL.Path != "/v1/orders" {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}

		if req.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type should be application/json")
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		var params map[string]string
		if err := json.Unmarshal(body, &params); err != nil {
			return nil, err
		}

		expected := map[string]string{
			"market":   "KRW-BTC",
			"side":     "bid",
			"price":    "10000",
			"ord_type": "price",
		}

		for key, value := range expected {
			if params[key] != value {
				t.Errorf(
					"unexpected %s: got=%s expected=%s",
					key,
					params[key],
					value,
				)
			}
		}

		query := url.Values{}
		for key, value := range params {
			query.Set(key, value)
		}

		verifyJWT(t, req, query.Encode())

		return jsonResponse(
			http.StatusCreated,
			`{"uuid":"test-order-uuid","market":"KRW-BTC","side":"bid","ord_type":"price","price":"10000","state":"wait"}`,
		), nil
	})

	order, err := client.CreateMarketBuyOrder(
		t.Context(),
		"KRW-BTC",
		"10000",
	)
	if err != nil {
		t.Fatalf("CreateMarketBuyOrder failed: %v", err)
	}

	if order.UUID != "test-order-uuid" {
		t.Errorf("unexpected order UUID: %s", order.UUID)
	}

	if order.Side != "bid" {
		t.Errorf("unexpected side: %s", order.Side)
	}

	if order.State != "wait" {
		t.Errorf("unexpected state: %s", order.State)
	}
}

func TestCreateMarketSellOrder(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", req.Method)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		var params map[string]string
		if err := json.Unmarshal(body, &params); err != nil {
			return nil, err
		}

		expected := map[string]string{
			"market":   "KRW-BTC",
			"side":     "ask",
			"volume":   "0.001",
			"ord_type": "market",
		}

		for key, value := range expected {
			if params[key] != value {
				t.Errorf(
					"unexpected %s: got=%s expected=%s",
					key,
					params[key],
					value,
				)
			}
		}

		query := url.Values{}
		for key, value := range params {
			query.Set(key, value)
		}

		verifyJWT(t, req, query.Encode())

		return jsonResponse(
			http.StatusCreated,
			`{"uuid":"sell-order-uuid","market":"KRW-BTC","side":"ask","ord_type":"market","volume":"0.001","state":"wait"}`,
		), nil
	})

	order, err := client.CreateMarketSellOrder(
		t.Context(),
		"KRW-BTC",
		"0.001",
	)
	if err != nil {
		t.Fatalf("CreateMarketSellOrder failed: %v", err)
	}

	if order.UUID != "sell-order-uuid" {
		t.Errorf("unexpected order UUID: %s", order.UUID)
	}

	if order.Side != "ask" {
		t.Errorf("unexpected side: %s", order.Side)
	}
}

func TestGetOrder(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", req.Method)
		}

		if req.URL.Path != "/v1/order" {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}

		if req.URL.Query().Get("uuid") != "test-order-uuid" {
			t.Errorf("unexpected UUID query: %s", req.URL.RawQuery)
		}

		verifyJWT(t, req, "uuid=test-order-uuid")

		return jsonResponse(
			http.StatusOK,
			`{"uuid":"test-order-uuid","market":"KRW-BTC","side":"bid","state":"done","executed_volume":"0.001"}`,
		), nil
	})

	order, err := client.GetOrder(
		t.Context(),
		"test-order-uuid",
	)
	if err != nil {
		t.Fatalf("GetOrder failed: %v", err)
	}

	if order.State != "done" {
		t.Errorf("unexpected order state: %s", order.State)
	}

	if order.ExecutedVolume != "0.001" {
		t.Errorf("unexpected executed volume: %s", order.ExecutedVolume)
	}
}

func TestCancelOrder(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", req.Method)
		}

		if req.URL.Path != "/v1/order" {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}

		if req.URL.Query().Get("uuid") != "test-order-uuid" {
			t.Errorf("unexpected UUID query: %s", req.URL.RawQuery)
		}

		verifyJWT(t, req, "uuid=test-order-uuid")

		return jsonResponse(
			http.StatusOK,
			`{"uuid":"test-order-uuid","market":"KRW-BTC","side":"bid","state":"cancel"}`,
		), nil
	})

	order, err := client.CancelOrder(
		t.Context(),
		"test-order-uuid",
	)
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}

	if order.State != "cancel" {
		t.Errorf("unexpected order state: %s", order.State)
	}
}

func TestOrderValidation(t *testing.T) {
	requestCount := 0

	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "empty market on buy",
			call: func() error {
				_, err := client.CreateMarketBuyOrder(
					t.Context(), "", "10000",
				)
				return err
			},
		},
		{
			name: "zero buy amount",
			call: func() error {
				_, err := client.CreateMarketBuyOrder(
					t.Context(), "KRW-BTC", "0",
				)
				return err
			},
		},
		{
			name: "invalid buy amount",
			call: func() error {
				_, err := client.CreateMarketBuyOrder(
					t.Context(), "KRW-BTC", "abc",
				)
				return err
			},
		},
		{
			name: "negative sell volume",
			call: func() error {
				_, err := client.CreateMarketSellOrder(
					t.Context(), "KRW-BTC", "-0.001",
				)
				return err
			},
		},
		{
			name: "empty order UUID",
			call: func() error {
				_, err := client.GetOrder(t.Context(), "")
				return err
			},
		},
		{
			name: "empty UUID on cancel",
			call: func() error {
				_, err := client.CancelOrder(t.Context(), "")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}

	if requestCount != 0 {
		t.Errorf(
			"invalid inputs should not send HTTP requests; got %d requests",
			requestCount,
		)
	}
}
