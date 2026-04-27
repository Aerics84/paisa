package stock

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func loadYahooTestConfig(t *testing.T) {
	t.Helper()

	err := config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: Europe/Berlin
`), "")
	require.NoError(t, err)
}

func TestGetHistoryReturnsErrorForEmptyYahooResult(t *testing.T) {
	loadYahooTestConfig(t)

	previousClient := yahooHTTPClient
	t.Cleanup(func() {
		yahooHTTPClient = previousClient
	})

	yahooHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(strings.NewReader(`{
				"chart": {
					"result": []
				}
			}`)),
		}, nil
	})}

	_, err := GetHistory("MISSING", "Missing Asset")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty yahoo result for MISSING")
}
