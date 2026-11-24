package server

// PortfolioDimension describes one rollup axis of the portfolio allocation
// response. It is a server-side helper, not part of the HTTP payload, so tygo
// excludes this file from the generated wire types.
type PortfolioDimension struct {
	Group      string
	GroupFn    func(CommodityBreakdown) string
	SubGroup   string
	SubGroupFn func(CommodityBreakdown) string
	FilterFn   func(CommodityBreakdown, int) bool
}
