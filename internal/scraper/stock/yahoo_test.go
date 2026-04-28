package stock

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func loadYahooTestConfig(t *testing.T) {
	t.Helper()

	loadYahooTestConfigWithCurrency(t, "EUR")
}

func loadYahooTestConfigWithCurrency(t *testing.T, currency string) {
	t.Helper()

	err := config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: `+currency+`
time_zone: Europe/Berlin
`), "")
	require.NoError(t, err)
}

func yahooJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func yahooChartJSON(currency string, dates []string, closes []float64) string {
	timestamps := make([]string, len(dates))
	closeValues := make([]string, len(closes))
	for i, date := range dates {
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			panic(err)
		}
		timestamps[i] = fmt.Sprintf("%d", parsed.Unix())
	}

	for i, close := range closes {
		closeValues[i] = fmt.Sprintf("%.6f", close)
	}

	return fmt.Sprintf(`{
		"chart": {
			"result": [{
				"timestamp": [%s],
				"indicators": { "quote": [{ "close": [%s] }] },
				"meta": { "currency": %q }
			}]
		}
	}`, strings.Join(timestamps, ", "), strings.Join(closeValues, ", "), currency)
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

func TestGetHistoryTrimsPricesBeforeExchangeHistory(t *testing.T) {
	loadYahooTestConfig(t)

	previousClient := yahooHTTPClient
	t.Cleanup(func() {
		yahooHTTPClient = previousClient
	})

	yahooHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.String(), "/AAPL?"):
			return yahooJSONResponse(yahooChartJSON("USD", []string{"2024-01-01", "2024-01-02", "2024-01-03"}, []float64{100, 110, 120})), nil
		case strings.Contains(req.URL.String(), "/USDEUR=X?"):
			return yahooJSONResponse(yahooChartJSON("EUR", []string{"2024-01-02", "2024-01-03"}, []float64{0.9, 0.8})), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	prices, err := GetHistory("AAPL", "Apple")
	require.NoError(t, err)
	require.Len(t, prices, 2)
	require.Equal(t, "2024-01-02", prices[0].Date.Format("2006-01-02"))
	require.True(t, prices[0].Value.Equal(decimal.RequireFromString("99")))
	require.Equal(t, "2024-01-03", prices[1].Date.Format("2006-01-02"))
	require.True(t, prices[1].Value.Equal(decimal.RequireFromString("96")))
}

func TestGetHistoryReturnsErrorWhenExchangeHistoryIsEmpty(t *testing.T) {
	loadYahooTestConfig(t)

	previousClient := yahooHTTPClient
	t.Cleanup(func() {
		yahooHTTPClient = previousClient
	})

	yahooHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.String(), "/AAPL?"):
			return yahooJSONResponse(yahooChartJSON("USD", []string{"2024-01-01"}, []float64{100})), nil
		case strings.Contains(req.URL.String(), "/USDEUR=X?"):
			return yahooJSONResponse(yahooChartJSON("EUR", []string{}, []float64{})), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	_, err := GetHistory("AAPL", "Apple")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing yahoo exchange rate data for USD")
}

func TestGetHistoryReturnsErrorWhenAllPricesPrecedeExchangeHistory(t *testing.T) {
	loadYahooTestConfig(t)

	previousClient := yahooHTTPClient
	t.Cleanup(func() {
		yahooHTTPClient = previousClient
	})

	yahooHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.String(), "/AAPL?"):
			return yahooJSONResponse(yahooChartJSON("USD", []string{"2024-01-01", "2024-01-02"}, []float64{100, 110})), nil
		case strings.Contains(req.URL.String(), "/USDEUR=X?"):
			return yahooJSONResponse(yahooChartJSON("EUR", []string{"2024-01-03"}, []float64{0.9})), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	_, err := GetHistory("AAPL", "Apple")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no importable yahoo price history for AAPL after trimming prices before first USD/EUR exchange rate on 2024-01-03")
}

func TestGetHistoryKeepsSameCurrencyHistoryUnchanged(t *testing.T) {
	loadYahooTestConfig(t)

	previousClient := yahooHTTPClient
	t.Cleanup(func() {
		yahooHTTPClient = previousClient
	})

	requests := 0
	yahooHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch {
		case strings.Contains(req.URL.String(), "/BMW.DE?"):
			return yahooJSONResponse(yahooChartJSON("EUR", []string{"2024-01-01", "2024-01-02"}, []float64{10, 11})), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	prices, err := GetHistory("BMW.DE", "BMW")
	require.NoError(t, err)
	require.Len(t, prices, 2)
	require.Equal(t, 1, requests)
	require.True(t, prices[0].Value.Equal(decimal.RequireFromString("10")))
	require.True(t, prices[1].Value.Equal(decimal.RequireFromString("11")))
}

func TestGetHistoryTrimsForeignCurrencyHistoryForNonEURDefault(t *testing.T) {
	loadYahooTestConfigWithCurrency(t, "INR")

	previousClient := yahooHTTPClient
	t.Cleanup(func() {
		yahooHTTPClient = previousClient
	})

	yahooHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.String(), "/AAPL?"):
			return yahooJSONResponse(yahooChartJSON("USD", []string{"2024-01-01", "2024-01-02"}, []float64{100, 110})), nil
		case strings.Contains(req.URL.String(), "/USDINR=X?"):
			return yahooJSONResponse(yahooChartJSON("INR", []string{"2024-01-02"}, []float64{80})), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	prices, err := GetHistory("AAPL", "Apple")
	require.NoError(t, err)
	require.Len(t, prices, 1)
	require.Equal(t, "2024-01-02", prices[0].Date.Format("2006-01-02"))
	require.True(t, prices[0].Value.Equal(decimal.RequireFromString("8800")))
}
