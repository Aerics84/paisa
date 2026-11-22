package pricesync

import "strings"

// SyncOptions controls a single price run.
type SyncOptions struct {
	// Full forces a complete refresh instead of an incremental one.
	Full bool
}

// Outcome reports the result of a price run.
type Outcome struct {
	// Success is true when every commodity synced without errors.
	Success bool
	// Partial is true when at least one commodity succeeded and at least one
	// failed.
	Partial bool
	// FailedCommodities lists the names of the commodities that failed.
	FailedCommodities []string
	// Error holds the joined per-commodity errors.
	Error string
	// Warning holds the joined non-fatal warnings.
	Warning string
}

// OutcomeFromResults builds an Outcome from the per-commodity sync result.
func OutcomeFromResults(succeeded, failed, errors, warnings []string) Outcome {
	return Outcome{
		Success:           len(failed) == 0,
		Partial:           len(succeeded) > 0 && len(failed) > 0,
		FailedCommodities: failed,
		Error:             strings.Join(errors, "\n"),
		Warning:           strings.Join(warnings, "\n"),
	}
}
