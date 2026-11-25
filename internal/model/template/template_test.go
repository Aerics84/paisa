package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

	store := config.NewStore()
	require.NoError(t, store.LoadFile(path))
	require.NoError(t, os.RemoveAll(dir))

	_, err := Upsert(store, "my-bank", "{{date}}")
	require.Error(t, err)
	require.Error(t, Delete(store, "my-bank"))
}

func customTemplates(templates []Template) []Template {
	custom := []Template{}
	for _, t := range templates {
		if t.TemplateType == Custom {
			custom = append(custom, t)
		}
	}
	return custom
}

func TestAllUsesInjectedStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paisa.yaml")
	require.NoError(t, os.WriteFile(path, []byte("journal_path: test.ledger\ndb_path: test.db\n"), 0644))

	store := config.NewStore()
	require.NoError(t, store.LoadFile(path))

	require.Empty(t, customTemplates(All(store)))

	_, err := Upsert(store, "my-bank", "{{date}}")
	require.NoError(t, err)

	templates := customTemplates(All(store))
	require.Len(t, templates, 1)
	assert.Equal(t, "my-bank", templates[0].Name)

	require.NoError(t, Delete(store, "my-bank"))
	assert.Empty(t, store.Get().ImportTemplates)
}

func TestConcurrentUpsertKeepsEveryTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paisa.yaml")
	require.NoError(t, os.WriteFile(path, []byte("journal_path: test.ledger\ndb_path: test.db\n"), 0644))

	store := config.NewStore()
	require.NoError(t, store.LoadFile(path))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("bank-%d", i)
			_, err := Upsert(store, name, "{{date}}")
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	assert.Len(t, store.Get().ImportTemplates, 8)
	assert.Len(t, customTemplates(All(store)), 8)
}
