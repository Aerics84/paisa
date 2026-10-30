package stock

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/google/btree"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/utils"
)

var UserAgents = []string{
	// Chrome
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",

	// # Firefox
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:135.0) Gecko/20100101 Firefox/135.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14.7; rv:135.0) Gecko/20100101 Firefox/135.0",
	"Mozilla/5.0 (X11; Linux i686; rv:135.0) Gecko/20100101 Firefox/135.0",

	// # Safari
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7_4) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Safari/605.1.15",

	// # Edge
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/131.0.2903.86",
}

type UserAgent struct {
	sync.Once
	name string
}

var agent UserAgent

func selectAgent() {
	agent.name = UserAgents[rand.Intn(len(UserAgents))]
}

type Quote struct {
	Close []float64
}

type Indicators struct {
	Quote []Quote
}

type Meta struct {
	Currency string
}

type Result struct {
	Timestamp  []int64
	Indicators Indicators
	Meta       Meta
}

// UnmarshalJSON drops missing closes and their timestamps at the Yahoo boundary.
func (r *Result) UnmarshalJSON(data []byte) error {
	var raw struct {
		Timestamp  []int64
		Meta       Meta
		Indicators struct {
			Quote []struct {
				Close []*float64
			}
		}
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*r = Result{
		Meta:       raw.Meta,
		Timestamp:  make([]int64, 0, len(raw.Timestamp)),
		Indicators: Indicators{Quote: make([]Quote, len(raw.Indicators.Quote))},
	}
	for i, timestamp := range raw.Timestamp {
		missing := false
		for _, quote := range raw.Indicators.Quote {
			if quote.Close[i] == nil {
				missing = true
				break
			}
		}
		if missing {
			continue
		}

		r.Timestamp = append(r.Timestamp, timestamp)
		for j, quote := range raw.Indicators.Quote {
			r.Indicators.Quote[j].Close = append(r.Indicators.Quote[j].Close, *quote.Close[i])
		}
	}
	return nil
}

type Chart struct {
	Result []Result
	Error  *struct {
		Code        string
		Description string
	}
}
type Response struct {
	Chart Chart
}

type ExchangePrice struct {
	Timestamp int64
	Close     float64
}

var yahooHTTPClient = http.DefaultClient

func (p ExchangePrice) Less(o btree.Item) bool {
	return p.Timestamp < (o.(ExchangePrice).Timestamp)
}

// normalizeYahooCurrency maps Yahoo's minor-unit currency codes to their
// ISO currency and returns the factor prices must be multiplied with.
func normalizeYahooCurrency(currency string) (string, float64) {
	switch currency {
	case "GBp", "GBX":
		return "GBP", 0.01
	default:
		return currency, 1
	}
}

func GetHistory(ticker string, commodityName string) ([]*price.Price, error) {
	log.Info("Fetching stock price history from Yahoo")
	response, err := getTicker(ticker)
	if err != nil {
		return nil, err
	}

	var prices []*price.Price
	result, err := firstYahooResult(ticker, response)
	if err != nil {
		return nil, err
	}
	quoteCurrency, scale := normalizeYahooCurrency(result.Meta.Currency)
	needExchangePrice := false
	var exchangePrice *btree.BTree
	var earliestExchangeTimestamp int64

	if !utils.IsCurrency(quoteCurrency) {
		needExchangePrice = true
		exchangeTicker := fmt.Sprintf("%s%s=X", quoteCurrency, config.DefaultCurrency())
		exchangeResponse, err := getTicker(exchangeTicker)
		if err != nil {
			return nil, err
		}

		exchangeResult, err := firstYahooResult(exchangeTicker, exchangeResponse)
		if err != nil {
			return nil, err
		}

		exchangePrice = btree.New(2)
		for i, t := range exchangeResult.Timestamp {
			if i >= len(exchangeResult.Indicators.Quote[0].Close) {
				break
			}

			close := exchangeResult.Indicators.Quote[0].Close[i]
			if math.IsNaN(close) || close == 0 {
				continue
			}

			exchangePrice.ReplaceOrInsert(ExchangePrice{Timestamp: t, Close: close})
		}

		if exchangePrice.Len() == 0 {
			return nil, fmt.Errorf("missing yahoo exchange rate data for %s", quoteCurrency)
		}

		earliestExchangeTimestamp = exchangePrice.Min().(ExchangePrice).Timestamp
	}

	skippedForMissingExchangeHistory := 0
	for i, timestamp := range result.Timestamp {
		if i >= len(result.Indicators.Quote[0].Close) {
			break
		}

		date := time.Unix(timestamp, 0)
		value := result.Indicators.Quote[0].Close[i] * scale
		if math.IsNaN(value) || value == 0 {
			continue
		}

		if needExchangePrice {
			if timestamp < earliestExchangeTimestamp {
				skippedForMissingExchangeHistory++
				continue
			}

			fxPrice := utils.BTreeDescendFirstLessOrEqual(exchangePrice, ExchangePrice{Timestamp: timestamp})
			if fxPrice.Close == 0 {
				return nil, fmt.Errorf("missing yahoo exchange rate near %s for %s", date.Format("2006-01-02"), quoteCurrency)
			}
			value = value * fxPrice.Close
		}

		price := price.Price{Date: date, CommodityType: config.Stock, CommodityID: ticker, CommodityName: commodityName, Value: decimal.NewFromFloat(value)}

		prices = append(prices, &price)
	}

	if skippedForMissingExchangeHistory > 0 {
		firstExchangeDate := time.Unix(earliestExchangeTimestamp, 0).Format("2006-01-02")
		log.Infof("Skipped %d yahoo price rows for %s before first %s/%s exchange rate on %s", skippedForMissingExchangeHistory, ticker, quoteCurrency, config.DefaultCurrency(), firstExchangeDate)
	}

	if len(prices) == 0 {
		if needExchangePrice {
			firstExchangeDate := time.Unix(earliestExchangeTimestamp, 0).Format("2006-01-02")
			return nil, fmt.Errorf("no importable yahoo price history for %s after trimming prices before first %s/%s exchange rate on %s", ticker, quoteCurrency, config.DefaultCurrency(), firstExchangeDate)
		}
		return nil, fmt.Errorf("empty yahoo price history for %s", ticker)
	}
	return prices, nil
}

func getTicker(ticker string) (*Response, error) {
	response, err := fetchTicker(ticker, false)
	if err != nil && retryable(err) {
		return fetchTicker(ticker, true)
	}
	return response, err
}

func fetchTicker(ticker string, refresh bool) (*Response, error) {
	crumb, err := yahooSessionState.getCrumb(refresh)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://query2.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=50y&crumb=%s", ticker, url.QueryEscape(crumb))
	req, err := newYahooRequest(url)
	if err != nil {
		return nil, err
	}

	resp, err := yahooSessionState.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, &statusError{StatusCode: resp.StatusCode, URL: url}
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response Response
	err = json.Unmarshal(respBytes, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// newYahooRequest builds a request with the browser user agent Yahoo expects.
// The cookie and crumb handshake is rate limited just like the chart endpoint,
// so the default Go user agent would be answered with HTTP 429 and every price
// sync would silently keep the previously stored quotes.
func newYahooRequest(rawURL string) (*http.Request, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	agent.Do(func() { selectAgent() })
	req.Header.Add("User-Agent", agent.name)

	return req, nil
}

type statusError struct {
	StatusCode int
	URL        string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("yahoo request to %s failed with status %d", e.URL, e.StatusCode)
}

func retryable(err error) bool {
	var statusErr *statusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
			return true
		}
	}
	return false
}

var yahooCookieURL = "https://fc.yahoo.com"
var yahooCrumbURL = "https://query1.finance.yahoo.com/v1/test/getcrumb"

type yahooSession struct {
	mu    sync.Mutex
	jar   http.CookieJar
	crumb string
}

var yahooSessionState = newYahooSession()

func newYahooSession() *yahooSession {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	return &yahooSession{jar: jar}
}

func (s *yahooSession) client() *http.Client {
	return &http.Client{
		Jar:       s.jar,
		Transport: yahooHTTPClient.Transport,
		Timeout:   30 * time.Second,
	}
}

// getCrumb fetches and caches the cookie/crumb pair Yahoo expects on chart
// requests. refresh forces a new handshake after an unauthorized response.
func (s *yahooSession) getCrumb(refresh bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.crumb != "" && !refresh {
		return s.crumb, nil
	}

	client := s.client()

	cookieReq, err := newYahooRequest(yahooCookieURL)
	if err != nil {
		return "", err
	}

	cookieResp, err := client.Do(cookieReq)
	if err != nil {
		return "", err
	}
	io.Copy(io.Discard, cookieResp.Body)
	cookieResp.Body.Close()

	crumbReq, err := newYahooRequest(yahooCrumbURL)
	if err != nil {
		return "", err
	}

	crumbResp, err := client.Do(crumbReq)
	if err != nil {
		return "", err
	}
	defer crumbResp.Body.Close()

	if crumbResp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, crumbResp.Body)
		return "", &statusError{StatusCode: crumbResp.StatusCode, URL: yahooCrumbURL}
	}

	crumbBytes, err := io.ReadAll(crumbResp.Body)
	if err != nil {
		return "", err
	}

	crumb := strings.TrimSpace(string(crumbBytes))
	if crumb == "" || strings.HasPrefix(crumb, "<") {
		return "", fmt.Errorf("invalid yahoo crumb response")
	}

	s.crumb = crumb
	return crumb, nil
}

func firstYahooResult(ticker string, response *Response) (Result, error) {
	if response == nil {
		return Result{}, fmt.Errorf("empty yahoo response for %s", ticker)
	}

	if response.Chart.Error != nil {
		return Result{}, fmt.Errorf("yahoo error for %s: %s", ticker, response.Chart.Error.Description)
	}

	if len(response.Chart.Result) == 0 {
		return Result{}, fmt.Errorf("empty yahoo result for %s", ticker)
	}

	result := response.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return Result{}, fmt.Errorf("missing yahoo quote data for %s", ticker)
	}

	return result, nil
}

type YahooPriceProvider struct {
}

func (p *YahooPriceProvider) Code() string {
	return "com-yahoo"
}

func (p *YahooPriceProvider) Label() string {
	return "Yahoo Finance"
}

func (p *YahooPriceProvider) Description() string {
	return "Supports a large set of stocks, ETFs, mutual funds, currencies, bonds, commodities, and cryptocurrencies. The stock price will be automatically converted to your default currency using the yahoo exchange rate."
}

func (p *YahooPriceProvider) AutoCompleteFields() []price.AutoCompleteField {
	return []price.AutoCompleteField{
		{Label: "Ticker", ID: "ticker", Help: "Stock ticker symbol, can be located on Yahoo's website. For example, AAPL is the ticker symbol for Apple Inc. (AAPL)", InputType: "text"},
	}
}

func (p *YahooPriceProvider) AutoComplete(db *gorm.DB, field string, filter map[string]string) []price.AutoCompleteItem {
	return []price.AutoCompleteItem{}
}

func (p *YahooPriceProvider) ClearCache(db *gorm.DB) {
}

func (p *YahooPriceProvider) GetPrices(code string, commodityName string) ([]*price.Price, error) {
	return GetHistory(code, commodityName)
}
