package indicator

import "fmt"

// RSI calculates the Relative Strength Index
// using Wilder's smoothing method.
func RSI(prices []float64, period int) (float64, error) {

	if period <= 0 {
		return 0, fmt.Errorf("period must be greater than zero")
	}

	if len(prices) < period+1 {
		return 0, fmt.Errorf(
			"insufficient data: required=%d, actual=%d",
			period+1,
			len(prices),
		)
	}

	gain := 0.0
	loss := 0.0

	// Calculate initial average gain and loss.
	for i := 1; i <= period; i++ {
		change := prices[i] - prices[i-1]

		if change > 0 {
			gain += change
		} else if change < 0 {
			loss += -change
		}
	}

	avgGain := gain / float64(period)
	avgLoss := loss / float64(period)

	// Apply Wilder's smoothing method.
	for i := period + 1; i < len(prices); i++ {
		change := prices[i] - prices[i-1]

		currentGain := 0.0
		currentLoss := 0.0

		if change > 0 {
			currentGain = change
		} else if change < 0 {
			currentLoss = -change
		}

		avgGain = (avgGain*float64(period-1) + currentGain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + currentLoss) / float64(period)
	}

	// Handle edge cases.
	if avgGain == 0 && avgLoss == 0 {
		return 50.0, nil
	}

	if avgLoss == 0 {
		return 100.0, nil
	}

	if avgGain == 0 {
		return 0.0, nil
	}

	rs := avgGain / avgLoss
	rsi := 100.0 - (100.0 / (1.0 + rs))

	return rsi, nil
}
