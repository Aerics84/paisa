package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClearPriceProviderCacheRejectsUnknownProvider(t *testing.T) {
	result, err := ClearPriceProviderCache(nil, "bogus")

	require.ErrorIs(t, err, errUnknownProvider)
	assert.Nil(t, result)
}

func TestGetPriceAutoCompletionsRejectsUnknownProvider(t *testing.T) {
	result, err := GetPriceAutoCompletions(nil, AutoCompleteRequest{Provider: "bogus"})

	require.ErrorIs(t, err, errUnknownProvider)
	assert.Nil(t, result)
}
