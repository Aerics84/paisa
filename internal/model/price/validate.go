package price

import (
	"fmt"
	"sort"
	"time"
)

// ValidateOptions controls how a provider response is validated before it is
// persisted.
type ValidateOptions struct {
	// Since is the start of the window the provider was asked for.
	Since time.Time
	// Now bounds plausible dates. Defaults to the current time when zero.
	Now time.Time
	// RequireSince rejects a response that does not overlap the requested
	// window, which usually means the provider returned stale or partial data.
	RequireSince bool
}

var earliestAllowedDate = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

// ValidatePrices checks a provider response and returns a deduplicated,
// chronologically sorted copy. It never mutates the input slice or its prices.
func ValidatePrices(prices []*Price, opts ValidateOptions) ([]*Price, error) {
	if len(prices) == 0 {
		return nil, fmt.Errorf("price: response contains no prices")
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	futureLimit := now.Add(48 * time.Hour)

	byInstant := make(map[int64]*Price, len(prices))
	for i, p := range prices {
		if p == nil {
			return nil, fmt.Errorf("price: response contains nil price at index %d", i)
		}

		if p.Date.IsZero() {
			return nil, fmt.Errorf("price: response contains price at index %d with zero date", i)
		}

		date := p.Date.UTC()
		if date.After(futureLimit) {
			return nil, fmt.Errorf("price: response contains price dated %s, which is in the future", date.Format("2006-01-02"))
		}
		if date.Before(earliestAllowedDate) {
			return nil, fmt.Errorf("price: response contains price dated %s, which is too old", date.Format("2006-01-02"))
		}
		if !p.Value.IsPositive() {
			return nil, fmt.Errorf("price: response contains non-positive value %s for %s", p.Value.String(), date.Format("2006-01-02"))
		}

		valid := *p
		byInstant[date.UnixNano()] = &valid
	}

	if opts.RequireSince && !opts.Since.IsZero() {
		overlaps := false
		for _, p := range byInstant {
			if !p.Date.Before(opts.Since) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			return nil, fmt.Errorf("price: response does not contain any price since %s", opts.Since.Format(time.RFC3339))
		}
	}

	validated := make([]*Price, 0, len(byInstant))
	for _, p := range byInstant {
		validated = append(validated, p)
	}
	sort.Slice(validated, func(i, j int) bool {
		return validated[i].Date.Before(validated[j].Date)
	})

	return validated, nil
}
