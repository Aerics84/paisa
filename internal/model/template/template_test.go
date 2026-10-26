package template

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestUpsertReturnsErrorWhenConfigCannotBeSaved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paisa.yaml")
	require.NoError(t, os.WriteFile(path, []byte("journal_path: test.ledger\ndb_path: test.db\n"), 0644))
	config.LoadConfigFile(path)
	require.NoError(t, os.RemoveAll(dir))

	_, err := Upsert("my-bank", "{{date}}")
	require.Error(t, err)
	require.Error(t, Delete("my-bank"))
}
