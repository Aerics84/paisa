package server

// HarvestResponse is the JSON body returned by GET /api/harvest.
type HarvestResponse struct {
	Harvestables map[string]Harvestable `json:"harvestables"`
}

// CapitalGainsResponse is the JSON body returned by GET /api/capital_gains.
type CapitalGainsResponse struct {
	CapitalGains map[string]CapitalGain   `json:"capital_gains"`
	Totals       map[string]FYCapitalGain `json:"totals"`
}

// GermanyTaxResponse is the JSON body returned by GET
// /api/capital_income_tax.
type GermanyTaxResponse struct {
	TaxYears map[string]GermanyTaxYear `json:"tax_years"`
}

// ScheduleALResponse is the JSON body returned by GET /api/schedule_al.
type ScheduleALResponse struct {
	ScheduleALs map[string]ScheduleAL `json:"schedule_als"`
}
