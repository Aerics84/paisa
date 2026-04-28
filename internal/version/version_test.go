package version

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringPrefersInjectedVersion(t *testing.T) {
	original := current
	current = ""
	t.Cleanup(func() {
		current = original
	})

	current = "0.7.6.1"
	assert.Equal(t, "0.7.6.1", String())
}

func TestResolveVersionPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	emptyVersion := filepath.Join(tempDir, "empty.VERSION")
	validVersion := filepath.Join(tempDir, "valid.VERSION")

	require.NoError(t, os.WriteFile(emptyVersion, []byte(" \n"), 0o600))
	require.NoError(t, os.WriteFile(validVersion, []byte("0.7.6.1\n"), 0o600))

	assert.Equal(t, "1.2.3", resolveVersion("1.2.3", []string{validVersion}))
	assert.Equal(t, "0.7.6.1", resolveVersion("", []string{
		filepath.Join(tempDir, "missing.VERSION"),
		emptyVersion,
		validVersion,
	}))
	assert.Equal(t, "dev", resolveVersion("", []string{filepath.Join(tempDir, "missing.VERSION")}))
}

func TestLookupVersionReturnsFirstReadableVersionFile(t *testing.T) {
	tempDir := t.TempDir()
	emptyVersion := filepath.Join(tempDir, "empty.VERSION")
	validVersion := filepath.Join(tempDir, "valid.VERSION")

	require.NoError(t, os.WriteFile(emptyVersion, []byte(" \n"), 0o600))
	require.NoError(t, os.WriteFile(validVersion, []byte("0.7.6.1\n"), 0o600))

	resolved, ok := lookupVersion([]string{
		filepath.Join(tempDir, "missing.VERSION"),
		emptyVersion,
		validVersion,
	})
	require.True(t, ok)
	assert.Equal(t, "0.7.6.1", resolved)
}

func TestLookupVersionReturnsFalseWhenNoVersionDataExists(t *testing.T) {
	resolved, ok := lookupVersion([]string{"missing.VERSION"})
	assert.False(t, ok)
	assert.Equal(t, "", resolved)
}

func TestDockerBuildsInjectVersionLdflags(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	expected := `-ldflags "-X github.com/ananthakumaran/paisa/internal/version.current=${VERSION}"`

	for _, dockerfile := range []string{"Dockerfile", "Dockerfile.demo"} {
		content, err := os.ReadFile(filepath.Join(repoRoot, dockerfile))
		require.NoError(t, err)
		assert.Contains(t, string(content), expected, dockerfile)
	}
}
