package indicator

import "fmt"

// SMA calculates the Simple Moving Average.
func SMA(prices []float64, period int) (float64, error) {

	if period <= 0 {
		return 0, fmt.Errorf("period must be greater than zero")
	}

	if len(prices) < period {
		return 0, fmt.Errorf(
			"insufficient data: required=%d, actual=%d",
			period,
			len(prices),
		)
	}

	sum := 0.0

	// Calculate the average of the latest N prices.
	for _, price := range prices[len(prices)-period:] {
		sum += price
	}

	return sum / float64(period), nil
}
