package market

import "time"

// Candle represents an OHLCV candlestick.
type Candle struct {
	Market      string
	Time        time.Time
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      float64
	TradeAmount float64
}

// IsClosed checks whether the candle interval has ended.
func (c Candle) IsClosed(unit int, now time.Time) bool {
	return !c.Time.Add(time.Duration(unit) * time.Minute).After(now)
}
