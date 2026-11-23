package metal

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func loadMetalTestConfig(t *testing.T) {
	t.Helper()

	err := config.LoadConfig([]byte(`
journal_path: main.ledger
db_path: paisa.db
default_currency: EUR
time_zone: Europe/Berlin
`), "")
	require.NoError(t, err)
}

func stubYahooMetalNow(t *testing.T, now string) {
	t.Helper()

	previousNow := yahooMetalNow
	yahooMetalNow = func() time.Time {
		return mustParseMetalTestTime(t, now)
	}
	t.Cleanup(func() {
		yahooMetalNow = previousNow
	})
}

func mustParseMetalTestTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return parsed
}

// stubYahooMetal installs a metal HTTP client that serves Yahoo chart requests
// with the given handler and answers ECB requests with a fixed exchange rate
// history.
func stubYahooMetal(t *testing.T, chartHandler func(req *http.Request) (*http.Response, error)) {
	t.Helper()

	previousClient := metalHTTPClient
	t.Cleanup(func() {
		metalHTTPClient = previousClient
	})

	metalHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "eurofxref-hist.xml") {
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`
<Envelope>
  <Cube>
    <Cube time="2026-11-15">
      <Cube currency="USD" rate="1.0000"/>
    </Cube>
    <Cube time="2026-11-20">
      <Cube currency="USD" rate="1.0000"/>
    </Cube>
  </Cube>
</Envelope>`)),
			}, nil
		}

		return chartHandler(req)
	})}
}

// yahooMetalJSONResponse builds a Yahoo chart response for the given dates and
// closing prices.
func yahooMetalJSONResponse(t *testing.T, dates []string, closes []float64) *http.Response {
	t.Helper()

	timestamps := make([]int64, 0, len(dates))
	for _, date := range dates {
		parsed, err := time.ParseInLocation("2006-01-02", date, config.TimeZone())
		require.NoError(t, err)
		timestamps = append(timestamps, parsed.Unix())
	}

	payload, err := json.Marshal(map[string]any{
		"chart": map[string]any{
			"result": []any{
				map[string]any{
					"timestamp": timestamps,
					"indicators": map[string]any{
						"quote": []any{
							map[string]any{"close": closes},
						},
					},
					"meta": map[string]any{"currency": "USD"},
				},
			},
		},
	})
	require.NoError(t, err)

	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(string(payload))),
	}
}

func TestNormalizeIndiaMetalPrice(t *testing.T) {
	assert.True(t, normalizeIndiaMetalPrice("gold-999", decimal.NewFromInt(152765)).Equal(decimal.NewFromInt(15276).Add(decimal.RequireFromString("0.5"))))
	assert.True(t, normalizeIndiaMetalPrice("silver-999", decimal.NewFromInt(234380)).Equal(decimal.RequireFromString("234.38")))
}

func TestEuPriceProviderGoldNormalization(t *testing.T) {
	loadMetalTestConfig(t)

	previousClient := metalHTTPClient
	t.Cleanup(func() {
		metalHTTPClient = previousClient
	})

	metalHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.String(), "GC=F"):
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`{
					"chart": {
						"result": [{
							"timestamp": [1704067200],
							"indicators": { "quote": [{ "close": [3110.34768] }] },
							"meta": { "currency": "USD" }
						}]
					}
				}`)),
			}, nil
		case strings.Contains(req.URL.String(), "eurofxref-hist.xml"):
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`
<Envelope>
  <Cube>
    <Cube time="2024-01-01">
      <Cube currency="USD" rate="1.0000"/>
    </Cube>
  </Cube>
</Envelope>`)),
			}, nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	prices, err := (&EuPriceProvider{}).GetPrices("gold-999", "GOLD")
	require.NoError(t, err)
	require.Len(t, prices, 1)
	assert.Equal(t, "2024-01-01", prices[0].Date.Format("2006-01-02"))
	assert.True(t, prices[0].Value.Round(6).Equal(decimal.RequireFromString("100.000000")))
}

func TestEuPriceProviderSilverNormalization(t *testing.T) {
	loadMetalTestConfig(t)

	previousClient := metalHTTPClient
	t.Cleanup(func() {
		metalHTTPClient = previousClient
	})

	metalHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.String(), "SI=F"):
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`{
					"chart": {
						"result": [{
							"timestamp": [1704153600],
							"indicators": { "quote": [{ "close": [31.1034768] }] },
							"meta": { "currency": "USD" }
						}]
					}
				}`)),
			}, nil
		case strings.Contains(req.URL.String(), "eurofxref-hist.xml"):
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`
<Envelope>
  <Cube>
    <Cube time="2024-01-01">
      <Cube currency="USD" rate="1.0000"/>
    </Cube>
    <Cube time="2024-01-02">
      <Cube currency="USD" rate="1.0000"/>
    </Cube>
  </Cube>
</Envelope>`)),
			}, nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	prices, err := (&EuPriceProvider{}).GetPrices("silver-999", "SILVER")
	require.NoError(t, err)
	require.Len(t, prices, 1)
	assert.Equal(t, "2024-01-02", prices[0].Date.Format("2006-01-02"))
	assert.True(t, prices[0].Value.Round(6).Equal(decimal.RequireFromString("1.000000")))
}

func TestEuPriceProviderSkipsInProgressCurrentDayBar(t *testing.T) {
	loadMetalTestConfig(t)
	stubYahooMetalNow(t, "2024-01-02T06:00:00Z")

	previousClient := metalHTTPClient
	t.Cleanup(func() {
		metalHTTPClient = previousClient
	})

	metalHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.String(), "GC=F"):
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`{
					"chart": {
						"result": [{
							"timestamp": [1704067200, 1704153600],
							"indicators": { "quote": [{ "close": [3110.34768, 6220.69536] }] },
							"meta": {
								"currency": "USD",
								"currentTradingPeriod": {
									"regular": { "start": 1704153600, "end": 1704196800, "gmtoffset": 0 }
								}
							}
						}]
					}
				}`)),
			}, nil
		case strings.Contains(req.URL.String(), "eurofxref-hist.xml"):
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`
<Envelope>
  <Cube>
    <Cube time="2024-01-01">
      <Cube currency="USD" rate="1.0000"/>
    </Cube>
    <Cube time="2024-01-02">
      <Cube currency="USD" rate="1.0000"/>
    </Cube>
  </Cube>
</Envelope>`)),
			}, nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	prices, err := (&EuPriceProvider{}).GetPrices("gold-999", "GOLD")
	require.NoError(t, err)
	require.Len(t, prices, 1)
	assert.Equal(t, "2024-01-01", prices[0].Date.Format("2006-01-02"))
	assert.True(t, prices[0].Value.Round(6).Equal(decimal.RequireFromString("100.000000")))
}

func TestEuPriceProviderKeepsCurrentDayBarAfterSessionEnd(t *testing.T) {
	loadMetalTestConfig(t)
	stubYahooMetalNow(t, "2024-01-02T13:00:00Z")

	previousClient := metalHTTPClient
	t.Cleanup(func() {
		metalHTTPClient = previousClient
	})

	metalHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.String(), "SI=F"):
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`{
					"chart": {
						"result": [{
							"timestamp": [1704067200, 1704153600],
							"indicators": { "quote": [{ "close": [31.1034768, 62.2069536] }] },
							"meta": {
								"currency": "USD",
								"currentTradingPeriod": {
									"regular": { "start": 1704153600, "end": 1704196800, "gmtoffset": 0 }
								}
							}
						}]
					}
				}`)),
			}, nil
		case strings.Contains(req.URL.String(), "eurofxref-hist.xml"):
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`
<Envelope>
  <Cube>
    <Cube time="2024-01-01">
      <Cube currency="USD" rate="1.0000"/>
    </Cube>
    <Cube time="2024-01-02">
      <Cube currency="USD" rate="1.0000"/>
    </Cube>
  </Cube>
</Envelope>`)),
			}, nil
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	})}

	prices, err := (&EuPriceProvider{}).GetPrices("silver-999", "SILVER")
	require.NoError(t, err)
	require.Len(t, prices, 2)
	assert.Equal(t, "2024-01-02", prices[1].Date.Format("2006-01-02"))
	assert.True(t, prices[1].Value.Round(6).Equal(decimal.RequireFromString("2.000000")))
}

func TestEuPriceProviderSinceUsesPeriodRange(t *testing.T) {
	loadMetalTestConfig(t)

	var chartURL string
	stubYahooMetal(t, func(req *http.Request) (*http.Response, error) {
		chartURL = req.URL.String()
		return yahooMetalJSONResponse(t, []string{"2026-11-20"}, []float64{2000}), nil
	})

	prices, err := (&EuPriceProvider{}).GetPricesSince("gold-999", "GOLD", time.Date(2026, 11, 15, 0, 0, 0, 0, config.TimeZone()))
	require.NoError(t, err)
	require.NotEmpty(t, prices)
	assert.Contains(t, chartURL, "period1=")
	assert.Contains(t, chartURL, "period2=")
	assert.NotContains(t, chartURL, "range=50y")
}

func TestEuPriceProviderFullHistoryKeepsRange(t *testing.T) {
	loadMetalTestConfig(t)

	var chartURL string
	stubYahooMetal(t, func(req *http.Request) (*http.Response, error) {
		chartURL = req.URL.String()
		return yahooMetalJSONResponse(t, []string{"2026-11-20"}, []float64{2000}), nil
	})

	prices, err := (&EuPriceProvider{}).GetPrices("gold-999", "GOLD")
	require.NoError(t, err)
	require.NotEmpty(t, prices)
	assert.Contains(t, chartURL, "range=50y")
	assert.NotContains(t, chartURL, "period1=")
}
