package metal

import (
	"io"
	"net/http"
	"strings"
	"testing"

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
