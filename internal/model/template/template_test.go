package template

import (
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllReturnsEmptySliceNotNil(t *testing.T) {
	templates := all(nil, fstest.MapFS{})
	assert.NotNil(t, templates)
	payload, err := json.Marshal(map[string]any{"templates": templates})
	require.NoError(t, err)
	assert.Equal(t, `{"templates":[]}`, string(payload))
}

func TestAllIncludesCustomTemplates(t *testing.T) {
	templates := all([]config.ImportTemplate{{Name: "my-bank", Content: "{{date}}"}}, fstest.MapFS{})
	require.Len(t, templates, 1)
	assert.Equal(t, "my-bank", templates[0].Name)
}
