package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func sampleManifest(importID, originalPath, contentSHA, importedAt string) Manifest {
	return Manifest{
		RecordType:    "import_manifest",
		ImportID:      importID,
		SourceType:    "bank",
		OriginalPath:  originalPath,
		ContentSHA256: contentSHA,
		ImportedAt:    importedAt,
		Status:        "staged",
		Parser: ParserInfo{
			Name:    "paisa-import",
			Version: "1",
		},
		RowCounts: RowCounts{Read: 10, Accepted: 9, Rejected: 1, Skipped: 0},
		Errors: []ImportError{{
			Code:          "row_rejected",
			Message:       "could not parse row",
			RecordLocator: "row:4",
		}},
	}
}

func assertNoTempFiles(t *testing.T, root string) {
	t.Helper()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)
		if info.IsDir() {
			return nil
		}
		if strings.Contains(info.Name(), ".tmp") {
			t.Errorf("temporary file was left behind: %s", path)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestImportIDIsStable(t *testing.T) {
	first := ImportID("icici_bank_statement", "test/statement.csv", "deadbeef")
	second := ImportID("icici_bank_statement", "test/statement.csv", "deadbeef")

	assert.Equal(t, first, second)
	assert.Len(t, first, 64)
	assert.Regexp(t, "^[0-9a-f]{64}$", first)
	// Pinned value keeps the provenance identifier stable across releases.
	assert.Equal(t, "3badd4c8717992ae8e8de0b2f7094860a754c0b0e09b86a8752cbfe41d1d5f90", first)

	assert.NotEqual(t, first, ImportID("icici_bank_statement", "test/other.csv", "deadbeef"))
	assert.NotEqual(t, first, ImportID("other", "test/statement.csv", "deadbeef"))
	assert.NotEqual(t, first, ImportID("icici_bank_statement", "test/statement.csv", "cafebabe"))
}

func TestWriteManifestAtomically(t *testing.T) {
	root := t.TempDir()
	m := sampleManifest(
		ImportID("bank", "statements/checking.csv", sha256Hex("statement")),
		"statements/checking.csv",
		sha256Hex("statement"),
		"2026-01-02T10:00:00Z",
	)
	records := []map[string]any{
		{"payee": "Coffee Shop", "amount": 4.5},
		{"payee": "Book Store", "amount": 12.0},
	}

	manifestPath, err := Write(root, m, records)
	require.NoError(t, err)
	expectedPath := filepath.Join(root, "imports", "bank", m.ImportID, "manifest.json")
	assert.Equal(t, expectedPath, manifestPath)

	if runtime.GOOS != "windows" {
		info, err := os.Stat(manifestPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
		dirInfo, err := os.Stat(filepath.Dir(manifestPath))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm())
	}

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var written Manifest
	require.NoError(t, json.Unmarshal(data, &written))
	assert.Equal(t, m, written)

	recordsData, err := os.ReadFile(filepath.Join(filepath.Dir(manifestPath), "records.json"))
	require.NoError(t, err)
	var writtenRecords []map[string]any
	require.NoError(t, json.Unmarshal(recordsData, &writtenRecords))
	assert.Equal(t, records, writtenRecords)
	assertNoTempFiles(t, root)

	// A second write replaces the existing files in place.
	m.Status = "succeeded"
	_, err = Write(root, m, records)
	require.NoError(t, err)
	data, err = os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &written))
	assert.Equal(t, "succeeded", written.Status)
	assertNoTempFiles(t, root)

	// Force the final rename to fail by making the destination a directory.
	// The temporary file created for the manifest must be cleaned up.
	require.NoError(t, os.Remove(manifestPath))
	require.NoError(t, os.Mkdir(manifestPath, 0700))
	_, err = Write(root, m, records)
	assert.Error(t, err)
	assertNoTempFiles(t, root)
}

func TestReadLatestManifestPerPath(t *testing.T) {
	root := t.TempDir()

	oldID := ImportID("bank", "statements/checking.csv", sha256Hex("old"))
	newID := ImportID("bank", "statements/checking.csv", sha256Hex("new"))
	savingsID := ImportID("bank", "statements/savings.csv", sha256Hex("savings"))

	_, err := Write(root, sampleManifest(oldID, "statements/checking.csv", sha256Hex("old"), "2026-01-01T09:00:00Z"), []string{"old"})
	require.NoError(t, err)
	_, err = Write(root, sampleManifest(newID, "statements/checking.csv", sha256Hex("new"), "2026-01-02T09:00:00Z"), []string{"new"})
	require.NoError(t, err)
	_, err = Write(root, sampleManifest(savingsID, "statements/savings.csv", sha256Hex("savings"), "2026-01-01T09:00:00Z"), []string{"savings"})
	require.NoError(t, err)

	latest, err := ReadLatest(root, "bank")
	require.NoError(t, err)
	require.Len(t, latest, 2)

	byPath := map[string]Manifest{}
	for _, m := range latest {
		byPath[m.OriginalPath] = m
	}
	assert.Equal(t, newID, byPath["statements/checking.csv"].ImportID)
	assert.Equal(t, savingsID, byPath["statements/savings.csv"].ImportID)

	// Missing source types are an empty history rather than an error.
	empty, err := ReadLatest(root, "unknown")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestVerifySourceDetectsChangedHash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "statements", "checking.csv")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("original statement"), 0600))

	m := sampleManifest(
		ImportID("bank", "statements/checking.csv", sha256Hex("original statement")),
		"statements/checking.csv",
		sha256Hex("original statement"),
		"2026-01-02T10:00:00Z",
	)
	require.NoError(t, VerifySource(root, m))

	m.ContentSHA256 = sha256Hex("tampered statement")
	err := VerifySource(root, m)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "hash")

	m.ContentSHA256 = sha256Hex("original statement")
	m.OriginalPath = "statements/missing.csv"
	assert.Error(t, VerifySource(root, m))
}

func TestVerifySourceRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.csv")
	require.NoError(t, os.WriteFile(outsidePath, []byte("outside statement"), 0600))

	relative, err := filepath.Rel(root, outsidePath)
	require.NoError(t, err)
	require.NotEmpty(t, relative)

	m := sampleManifest(
		ImportID("bank", filepath.ToSlash(relative), sha256Hex("outside statement")),
		filepath.ToSlash(relative),
		sha256Hex("outside statement"),
		"2026-01-02T10:00:00Z",
	)
	err = VerifySource(root, m)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "outside")

	// Absolute paths are rejected even when they point at an existing file.
	m.OriginalPath = outsidePath
	err = VerifySource(root, m)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "outside")
}

func TestManifestJSONRoundTrip(t *testing.T) {
	original := Manifest{
		RecordType:    "import_manifest",
		ImportID:      ImportID("bank", "statements/checking.csv", sha256Hex("statement")),
		SourceType:    "bank",
		OriginalPath:  "statements/checking.csv",
		ContentSHA256: sha256Hex("statement"),
		ImportedAt:    "2026-01-02T10:00:00Z",
		Status:        "succeeded_with_errors",
		Parser:        ParserInfo{Name: "paisa-import", Version: "1"},
		RowCounts:     RowCounts{Read: 20, Accepted: 18, Rejected: 2, Skipped: 0},
		Errors: []ImportError{
			{Code: "row_rejected", Message: "invalid amount", RecordLocator: "row:7"},
			{Code: "row_rejected", Message: "invalid date", RecordLocator: "row:9"},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Manifest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)

	assert.Contains(t, string(data), `"record_type":"import_manifest"`)
	assert.Contains(t, string(data), `"record_locator":"row:7"`)
}

func TestUpdateStatusAtomicallyPreservesRecords(t *testing.T) {
	root := t.TempDir()
	id := ImportID("bank", "statements/checking.csv", sha256Hex("statement"))
	records := []map[string]any{
		{"payee": "Coffee Shop", "amount": 4.5, "balance": 120.5},
	}

	_, err := Write(root, sampleManifest(id, "statements/checking.csv", sha256Hex("statement"), "2026-01-02T10:00:00Z"), records)
	require.NoError(t, err)

	updated, err := MarkCommitted(root, "bank", id)
	require.NoError(t, err)
	assert.Equal(t, "committed", updated.Status)

	data, err := os.ReadFile(filepath.Join(root, "imports", "bank", id, "manifest.json"))
	require.NoError(t, err)
	var written Manifest
	require.NoError(t, json.Unmarshal(data, &written))
	assert.Equal(t, "committed", written.Status)

	recordsData, err := os.ReadFile(filepath.Join(root, "imports", "bank", id, "records.json"))
	require.NoError(t, err)
	var writtenRecords []map[string]any
	require.NoError(t, json.Unmarshal(recordsData, &writtenRecords))
	assert.Equal(t, records, writtenRecords)
	assertNoTempFiles(t, root)

	latest, err := ReadLatest(root, "bank")
	require.NoError(t, err)
	require.Len(t, latest, 1)
	assert.Equal(t, "committed", latest[0].Status)
}

func TestUpdateStatusRejectsMissingManifest(t *testing.T) {
	root := t.TempDir()
	_, err := MarkCommitted(root, "bank", ImportID("bank", "missing.csv", sha256Hex("missing")))
	require.Error(t, err)
}

func TestFindSourceType(t *testing.T) {
	root := t.TempDir()
	id := ImportID("csv", "statements/checking.csv", sha256Hex("statement"))
	_, err := Write(root, sampleManifest(id, "statements/checking.csv", sha256Hex("statement"), "2026-01-02T10:00:00Z"), []string{})
	require.NoError(t, err)

	sourceType, err := FindSourceType(root, id)
	require.NoError(t, err)
	assert.Equal(t, "bank", sourceType)

	_, err = FindSourceType(root, ImportID("csv", "other.csv", sha256Hex("other")))
	require.Error(t, err)
}
