package goal

import (
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/shopspring/decimal"
)

func retirementPosting(date string, amount int64) posting.Posting {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}

	return posting.Posting{
		Date:   parsed,
		Amount: decimal.NewFromInt(amount),
	}
}

func TestCalculateMonthlyContributionUsesLastTwelveCompletedMonths(t *testing.T) {
	postings := []posting.Posting{
		retirementPosting("2025-03-15", 300),
		retirementPosting("2025-04-10", 100),
		retirementPosting("2025-05-10", 200),
		retirementPosting("2026-03-10", 300),
		retirementPosting("2026-04-10", 400),
	}

	got := calculateMonthlyContribution(postings, time.Date(2026, time.April, 28, 0, 0, 0, 0, time.UTC))

	want := decimal.NewFromInt(50)
	if !got.Equal(want) {
		t.Fatalf("expected monthly contribution %s, got %s", want, got)
	}
}

func TestCalculateMonthlyContributionIgnoresNonPositiveAmounts(t *testing.T) {
	postings := []posting.Posting{
		retirementPosting("2025-06-10", -100),
		retirementPosting("2025-07-10", 0),
	}

	got := calculateMonthlyContribution(postings, time.Date(2026, time.April, 28, 0, 0, 0, 0, time.UTC))

	if !got.Equal(decimal.Zero) {
		t.Fatalf("expected zero monthly contribution, got %s", got)
	}
}
