package pricesync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutcomeFromResults(t *testing.T) {
	got := OutcomeFromResults([]string{"AAPL"}, []string{"NFLX"}, []string{"NFLX: boom"}, []string{"CII: offline"})
	assert.False(t, got.Success)
	assert.True(t, got.Partial)
	assert.Equal(t, []string{"NFLX"}, got.FailedCommodities)
	assert.Equal(t, "NFLX: boom", got.Error)
	assert.Equal(t, "CII: offline", got.Warning)
}
