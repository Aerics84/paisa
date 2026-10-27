package server

import "github.com/shopspring/decimal"

func init() {
	// The HTTP API emits decimal amounts as JSON numbers. The Svelte client
	// relies on numbers, not quoted strings. This is process-wide and covers
	// every program that imports internal/server (CLI serve, desktop, tests),
	// not decimal marshaling in unrelated packages.
	decimal.MarshalJSONWithoutQuotes = true
}
