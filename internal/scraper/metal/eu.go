package metal

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/google/btree"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	ecbUSDHistoryURL    = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-hist.xml"
	yahooChartURLFormat = "https://query2.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=50y"
)

var troyOunceInGrams = decimal.RequireFromString("31.1034768")

type EuPriceProvider struct {
}

func (p *EuPriceProvider) Code() string {
	return "com-yahoo-eu-metal"
}

func (p *EuPriceProvider) Label() string {
	return "Yahoo Metals Europe"
}

func (p *EuPriceProvider) Description() string {
	return "Supports gold-999 and silver-999 for European setups. Prices are sourced from Yahoo Finance in USD per troy ounce and converted to EUR per gram using ECB reference FX rates."
}

func (p *EuPriceProvider) AutoCompleteFields() []price.AutoCompleteField {
	return []price.AutoCompleteField{
		{Label: "Metal", ID: "metal", Help: "Supported codes: gold-999, silver-999."},
	}
}

func (p *EuPriceProvider) AutoComplete(db *gorm.DB, field string, filter map[string]string) []price.AutoCompleteItem {
	return []price.AutoCompleteItem{
		{Label: "Gold 999", ID: "gold-999"},
		{Label: "Silver 999", ID: "silver-999"},
	}
}

func (p *EuPriceProvider) ClearCache(db *gorm.DB) {
}

func (p *EuPriceProvider) GetPrices(code string, commodityName string) ([]*price.Price, error) {
	ticker, ok := yahooMetalTicker(code)
	if !ok {
		return nil, fmt.Errorf("unsupported EU metal code: %s", code)
	}

	log.Infof("Fetching EU metal price history from Yahoo Finance for %s", code)
	chart, err := getYahooChart(ticker)
	if err != nil {
		return nil, err
	}

	if len(chart.Chart.Result) == 0 {
		return nil, fmt.Errorf("empty yahoo response for %s", ticker)
	}

	result := chart.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("missing yahoo quote data for %s", ticker)
	}

	if result.Meta.Currency != "USD" {
		return nil, fmt.Errorf("unexpected yahoo metal currency %s for %s", result.Meta.Currency, ticker)
	}

	usdPerEUR, err := getECBUSDPerEUR()
	if err != nil {
		return nil, err
	}

	var prices []*price.Price
	for i, timestamp := range result.Timestamp {
		if i >= len(result.Indicators.Quote[0].Close) {
			break
		}

		close := result.Indicators.Quote[0].Close[i]
		if math.IsNaN(close) || close == 0 {
			continue
		}

		date := time.Unix(timestamp, 0).In(config.TimeZone())
		usdRate := utils.BTreeDescendFirstLessOrEqual(usdPerEUR, ecbRate{Date: date})
		if usdRate.USDPerEUR.IsZero() {
			return nil, fmt.Errorf("missing ECB USD/EUR rate for %s", date.Format("2006-01-02"))
		}

		eurPerOunce := decimal.NewFromFloat(close).Div(usdRate.USDPerEUR)
		eurPerGram := eurPerOunce.Div(troyOunceInGrams)

		prices = append(prices, &price.Price{
			Date:          date,
			CommodityType: config.Metal,
			CommodityID:   code,
			CommodityName: commodityName,
			Value:         eurPerGram,
		})
	}

	return prices, nil
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
			Meta struct {
				Currency string `json:"currency"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

func getYahooChart(ticker string) (*yahooChartResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(yahooChartURLFormat, ticker), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")

	resp, err := metalHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected yahoo status code: %d, body: %s", resp.StatusCode, string(respBytes))
	}

	var response yahooChartResponse
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, err
	}

	if len(response.Chart.Result) == 0 {
		return nil, fmt.Errorf("empty yahoo metal response for %s", ticker)
	}

	if len(response.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, fmt.Errorf("missing yahoo metal quote data for %s", ticker)
	}

	return &response, nil
}

type ecbEnvelope struct {
	Cube ecbOuterCube `xml:"Cube"`
}

type ecbOuterCube struct {
	Cube []ecbTimeCube `xml:"Cube"`
}

type ecbTimeCube struct {
	Time string        `xml:"time,attr"`
	Cube []ecbRateCube `xml:"Cube"`
}

type ecbRateCube struct {
	Currency string `xml:"currency,attr"`
	Rate     string `xml:"rate,attr"`
}

type ecbRate struct {
	Date      time.Time
	USDPerEUR decimal.Decimal
}

func (r ecbRate) Less(o btree.Item) bool {
	return r.Date.Before(o.(ecbRate).Date)
}

func getECBUSDPerEUR() (*btree.BTree, error) {
	resp, err := metalHTTPClient.Get(ecbUSDHistoryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected ECB status code: %d, body: %s", resp.StatusCode, string(respBytes))
	}

	var envelope ecbEnvelope
	if err := xml.Unmarshal(respBytes, &envelope); err != nil {
		return nil, err
	}

	tree := btree.New(2)
	for _, timeCube := range envelope.Cube.Cube {
		date, err := time.ParseInLocation("2006-01-02", timeCube.Time, config.TimeZone())
		if err != nil {
			return nil, err
		}

		for _, rateCube := range timeCube.Cube {
			if rateCube.Currency != "USD" {
				continue
			}

			rate, err := decimal.NewFromString(rateCube.Rate)
			if err != nil {
				return nil, err
			}

			tree.ReplaceOrInsert(ecbRate{Date: date, USDPerEUR: rate})
			break
		}
	}

	return tree, nil
}

func yahooMetalTicker(code string) (string, bool) {
	switch code {
	case "gold-999":
		return "GC=F", true
	case "silver-999":
		return "SI=F", true
	default:
		return "", false
	}
}
