package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// backupTestRoot is the config directory shared by the backup tests. The
// config package only records the config path once per process, so every test
// has to write its config file to the same location.
var backupTestRoot string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "paisa-backup-tests-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create backup test directory:", err)
		os.Exit(1)
	}
	backupTestRoot = dir

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func backupTestConfigPath() string {
	return filepath.Join(backupTestRoot, "paisa.yaml")
}

func yamlPath(path string) string {
	return strconv.Quote(filepath.ToSlash(path))
}

func writeBackupTestConfig(t *testing.T, journalPath string, dbPath string, ledgerCLI string, extra string) {
	t.Helper()

	content := fmt.Sprintf(
		"journal_path: %s\ndb_path: %s\ndefault_currency: EUR\ntime_zone: UTC\nledger_cli: %s\n%s",
		yamlPath(journalPath),
		yamlPath(dbPath),
		ledgerCLI,
		extra,
	)

	configPath := backupTestConfigPath()
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))
	require.NoError(t, config.LoadConfig([]byte(content), configPath))
}

func readBackupEntries(t *testing.T, zipData []byte) map[string]string {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	require.NoError(t, err)

	entries := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		entries[file.Name] = string(content)
	}

	return entries
}

func TestGenerateBackupZipContainsExpectedFilesAndExcludesDB(t *testing.T) {
	journalDir := t.TempDir()
	journalPath := filepath.Join(journalDir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte("2026-05-01 * Sample\n  Assets:Cash  100\n  Income:Salary\n"), 0644))

	dbPath := filepath.Join(journalDir, "paisa.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("SQLITE_DUMMY_DATA"), 0644))

	sheetDir := filepath.Join(journalDir, "nested")
	require.NoError(t, os.MkdirAll(sheetDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sheetDir, "budget.paisa"), []byte("Sheet Data"), 0644))

	writeBackupTestConfig(t, journalPath, dbPath, "ledger", "")

	zipData, filename, err := GenerateBackupZip(false)
	require.NoError(t, err)
	assert.NotEmpty(t, zipData)
	assert.True(t, strings.HasPrefix(filename, "paisa-backup-"), "filename %q should start with paisa-backup-", filename)

	entries := readBackupEntries(t, zipData)

	assert.Contains(t, entries, "journals/main.ledger")
	assert.Contains(t, entries, "config/paisa.yaml")
	assert.Contains(t, entries, "sheets/nested/budget.paisa")

	for name := range entries {
		lowerName := strings.ToLower(name)
		assert.NotContains(t, lowerName, "paisa.db", "database file must not be part of the backup")
		assert.False(t, strings.HasSuffix(lowerName, ".db"), "database file %q must not be part of the backup", name)
	}
}

func TestGenerateBackupZipRedactsSecrets(t *testing.T) {
	journalDir := t.TempDir()
	journalPath := filepath.Join(journalDir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte("2026-05-01 * Sample\n  Assets:Cash  100\n  Income:Salary\n"), 0644))

	passwordHash := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	extra := fmt.Sprintf("user_accounts:\n  - username: admin\n    password: %s\n", strconv.Quote(passwordHash))

	writeBackupTestConfig(t, journalPath, filepath.Join(journalDir, "paisa.db"), "ledger", extra)

	zipData, _, err := GenerateBackupZip(false)
	require.NoError(t, err)

	entries := readBackupEntries(t, zipData)
	configContent, ok := entries["config/paisa.yaml"]
	require.True(t, ok, "config/paisa.yaml must be part of the backup")

	assert.Contains(t, configContent, "[REDACTED]")
	assert.NotContains(t, configContent, passwordHash)
	assert.Contains(t, configContent, "journal_path")
	assert.Contains(t, configContent, "default_currency")
}

func TestGenerateBackupZipIncludesSecretsWhenRequested(t *testing.T) {
	journalDir := t.TempDir()
	journalPath := filepath.Join(journalDir, "main.ledger")
	require.NoError(t, os.WriteFile(journalPath, []byte("2026-05-01 * Sample\n  Assets:Cash  100\n  Income:Salary\n"), 0644))

	passwordHash := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	extra := fmt.Sprintf("user_accounts:\n  - username: admin\n    password: %s\n", strconv.Quote(passwordHash))

	writeBackupTestConfig(t, journalPath, filepath.Join(journalDir, "paisa.db"), "ledger", extra)

	zipData, _, err := GenerateBackupZip(true)
	require.NoError(t, err)

	entries := readBackupEntries(t, zipData)
	configContent, ok := entries["config/paisa.yaml"]
	require.True(t, ok, "config/paisa.yaml must be part of the backup")

	assert.Contains(t, configContent, passwordHash)
	assert.NotContains(t, configContent, "[REDACTED]")
}

func TestGenerateBackupZipSkipsUnsafePaths(t *testing.T) {
	journalDir := t.TempDir()
	outsideDir := t.TempDir()

	outsideJournal := filepath.Join(outsideDir, "secret.beancount")
	require.NoError(t, os.WriteFile(outsideJournal, []byte("; OUTSIDE_SECRET_MARKER\n"), 0644))

	relativeOutside, err := filepath.Rel(journalDir, outsideJournal)
	require.NoError(t, err)

	journalPath := filepath.Join(journalDir, "main.beancount")
	require.NoError(t, os.WriteFile(journalPath, []byte(fmt.Sprintf("include %s\n", strconv.Quote(filepath.ToSlash(relativeOutside)))), 0644))

	writeBackupTestConfig(t, journalPath, filepath.Join(journalDir, "paisa.db"), "beancount", "")

	zipData, _, err := GenerateBackupZip(false)
	require.NoError(t, err)

	entries := readBackupEntries(t, zipData)
	assert.Contains(t, entries, "journals/main.beancount")

	for name, content := range entries {
		assert.NotContains(t, name, "secret", "file outside the allowed roots must not be included")
		assert.NotContains(t, content, "OUTSIDE_SECRET_MARKER", "content outside the allowed roots must not be included")
	}
}
