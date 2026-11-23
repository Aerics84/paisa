package stock

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlphaVantageRangeUsesCompactOutput(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: USD
time_zone: UTC
`), ""))

	previous := alphaVantageHTTPClient
	t.Cleanup(func() { alphaVantageHTTPClient = previous })

	var requestedURL string
	alphaVantageHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedURL = req.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"Time Series (Daily)": {
					"2026-11-20": {"4. close": "100"},
					"2026-11-21": {"4. close": "101"}
				}
			}`)),
		}, nil
	})}

	prices, err := (&AlphaVantagePriceProvider{}).GetPricesSince("key:AAPL:USD", "AAPL", time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, prices, 2)
	assert.Contains(t, requestedURL, "outputsize=compact")
	assert.NotContains(t, requestedURL, "outputsize=full")
}
