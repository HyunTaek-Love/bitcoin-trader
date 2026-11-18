package indicator

import (
	"math"
	"testing"
)

func TestSMA(t *testing.T) {

	tests := []struct {
		name    string
		prices  []float64
		period  int
		want    float64
		wantErr bool
	}{
		{
			name:   "normal",
			prices: []float64{100, 110, 120, 130, 140},
			period: 3,
			want:   130,
		},
		{
			name:    "insufficient data",
			prices:  []float64{100, 110},
			period:  3,
			wantErr: true,
		},
		{
			name:    "invalid period",
			prices:  []float64{100, 110, 120},
			period:  0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := SMA(tt.prices, tt.period)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("got %.6f, want %.6f", got, tt.want)
			}
		})
	}
}

func TestRSI(t *testing.T) {

	rising := make([]float64, 15)
	falling := make([]float64, 15)
	flat := make([]float64, 15)

	for i := 0; i < 15; i++ {
		rising[i] = float64(i + 1)
		falling[i] = float64(15 - i)
		flat[i] = 100
	}

	tests := []struct {
		name    string
		prices  []float64
		want    float64
		wantErr bool
	}{
		{
			name:   "rising prices",
			prices: rising,
			want:   100,
		},
		{
			name:   "falling prices",
			prices: falling,
			want:   0,
		},
		{
			name:   "flat prices",
			prices: flat,
			want:   50,
		},
		{
			name:    "insufficient data",
			prices:  []float64{100, 101, 102},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := RSI(tt.prices, 14)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("got %.6f, want %.6f", got, tt.want)
			}
		})
	}
}

func TestCalculate(t *testing.T) {

	prices := make([]float64, 60)

	for i := range prices {
		prices[i] = float64(i + 1)
	}

	result, err := Calculate(prices)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if math.Abs(result.SMA20-50.5) > 1e-9 {
		t.Errorf("SMA20: got %.2f, want 50.5", result.SMA20)
	}

	if math.Abs(result.SMA50-35.5) > 1e-9 {
		t.Errorf("SMA50: got %.2f, want 35.5", result.SMA50)
	}

	if math.Abs(result.RSI14-100.0) > 1e-9 {
		t.Errorf("RSI14: got %.2f, want 100", result.RSI14)
	}
}
