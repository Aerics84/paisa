package currency

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	original := BaseURL
	BaseURL = server.URL
	t.Cleanup(func() {
		BaseURL = original
		server.Close()
	})
}

func TestGetHistoryWalksBackUntilEmpty(t *testing.T) {
	until := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	var ranges []string

	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "USD", r.URL.Query().Get("base"))
		assert.Equal(t, "INR", r.URL.Query().Get("symbols"))
		ranges = append(ranges, strings.TrimPrefix(r.URL.Path, "/"))

		switch len(ranges) {
		case 1:
			fmt.Fprint(w, `{"amount":1,"base":"USD","rates":{"2026-10-30":{"INR":94.955245},"2026-10-29":{"INR":95.269093}}}`)
		case 2:
			fmt.Fprint(w, `{"amount":1,"base":"USD","rates":{"2025-01-02":{"INR":85.5}}}`)
		default:
			http.Error(w, `{"error":"No published rates in that range"}`, http.StatusNotFound)
		}
	})

	prices, err := GetHistory("usd", "INR", "USD", until)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"2025-10-31..2026-10-31",
		"2024-10-30..2025-10-30",
		"2023-10-30..2024-10-29",
	}, ranges)

	require.Len(t, prices, 3)
	assert.Equal(t, "2025-01-02", prices[0].Date.Format("2006-01-02"))
	assert.Equal(t, "85.5", prices[0].Value.String())
	assert.Equal(t, "2026-10-29", prices[1].Date.Format("2006-01-02"))
	assert.Equal(t, "2026-10-30", prices[2].Date.Format("2006-01-02"))
	assert.Equal(t, "94.955245", prices[2].Value.String())
	for _, p := range prices {
		assert.Equal(t, "USD", p.CommodityID)
		assert.Equal(t, "USD", p.CommodityName)
	}
}

func TestGetHistorySkipsMissingSymbols(t *testing.T) {
	until := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(strings.TrimPrefix(r.URL.Path, "/"), "2025-10-31") {
			// A date where the symbol is missing, a date with a zero
			// rate, and a date with a usable rate.
			fmt.Fprint(w, `{"amount":1,"base":"USD","rates":{"2026-10-30":{},"2026-10-29":{"INR":0},"2026-10-28":{"INR":94.955245}}}`)
			return
		}
		http.Error(w, `{"error":"No published rates in that range"}`, http.StatusNotFound)
	})

	prices, err := GetHistory("USD", "INR", "USD", until)
	require.NoError(t, err)
	require.Len(t, prices, 1)
	assert.Equal(t, "2026-10-28", prices[0].Date.Format("2006-01-02"))
	assert.Equal(t, "94.955245", prices[0].Value.String())
}

func TestGetHistoryStopsOnNotFound(t *testing.T) {
	until := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	requests := 0

	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			fmt.Fprint(w, `{"amount":1,"base":"USD","rates":{"2026-10-30":{"INR":94.955245}}}`)
			return
		}
		http.Error(w, `{"error":"No published rates in that range"}`, http.StatusNotFound)
	})

	prices, err := GetHistory("USD", "INR", "USD", until)
	require.NoError(t, err)
	assert.Equal(t, 2, requests)
	require.Len(t, prices, 1)
	assert.Equal(t, "2026-10-30", prices[0].Date.Format("2006-01-02"))
}

func TestGetHistoryRejectsInvalidResponse(t *testing.T) {
	until := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"rates":`)
	})

	_, err := GetHistory("USD", "INR", "USD", until)
	require.Error(t, err)
}

func TestGetHistoryValidatesCurrencyPair(t *testing.T) {
	until := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	t.Run("same currency", func(t *testing.T) {
		withServer(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("unexpected request: %s", r.URL)
		})
		_, err := GetHistory("INR", "INR", "INR", until)
		require.Error(t, err)
	})

	t.Run("empty currency", func(t *testing.T) {
		withServer(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("unexpected request: %s", r.URL)
		})
		_, err := GetHistory(" ", "INR", "USD", until)
		require.Error(t, err)
	})

	t.Run("no published rates", func(t *testing.T) {
		withServer(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"No published rates in that range"}`, http.StatusNotFound)
		})
		_, err := GetHistory("ARS", "INR", "ARS", until)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ARS/INR")
	})

	t.Run("server error", func(t *testing.T) {
		withServer(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		})
		_, err := GetHistory("USD", "INR", "USD", until)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})
}

func TestPriceProviderCode(t *testing.T) {
	assert.Equal(t, "com-allratestoday", (&PriceProvider{}).Code())
}
