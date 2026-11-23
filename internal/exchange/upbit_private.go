package exchange

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const privateBaseURL = "https://api.upbit.com/v1"

// UpbitPrivateClient handles authenticated Upbit API requests.
type UpbitPrivateClient struct {
	accessKey  string
	secretKey  string
	httpClient *http.Client
}

// Account represents an Upbit account balance.
type Account struct {
	Currency            string `json:"currency"`
	Balance             string `json:"balance"`
	Locked              string `json:"locked"`
	AvgBuyPrice         string `json:"avg_buy_price"`
	AvgBuyPriceModified bool   `json:"avg_buy_price_modified"`
	UnitCurrency        string `json:"unit_currency"`
}

// NewUpbitPrivateClient creates an authenticated client.
func NewUpbitPrivateClient(
	accessKey string,
	secretKey string,
) (*UpbitPrivateClient, error) {
	if accessKey == "" {
		return nil, fmt.Errorf("access key is empty")
	}

	if secretKey == "" {
		return nil, fmt.Errorf("secret key is empty")
	}

	return &UpbitPrivateClient{
		accessKey: accessKey,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// createJWT creates an Upbit API authentication token.

func (c *UpbitPrivateClient) createJWT(queryString string) (string, error) {
	claims := jwt.MapClaims{
		"access_key": c.accessKey,
		"nonce":      uuid.NewString(),
	}

	if queryString != "" {
		hash := sha512.Sum512([]byte(queryString))

		claims["query_hash"] = hex.EncodeToString(hash[:])
		claims["query_hash_alg"] = "SHA512"
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS512,
		claims,
	)

	signedToken, err := token.SignedString([]byte(c.secretKey))
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}

	return signedToken, nil
}

// GetAccounts retrieves account balances.
func (c *UpbitPrivateClient) GetAccounts(
	ctx context.Context,
) ([]Account, error) {
	token, err := c.createJWT("")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		privateBaseURL+"/accounts",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create account request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)

		return nil, fmt.Errorf(
			"upbit account API error: status=%d body=%s",
			resp.StatusCode,
			string(body),
		)
	}

	var accounts []Account

	decoder := json.NewDecoder(
		io.LimitReader(resp.Body, 1024*1024),
	)

	if err := decoder.Decode(&accounts); err != nil {
		return nil, fmt.Errorf("decode accounts: %w", err)
	}

	return accounts, nil
}
