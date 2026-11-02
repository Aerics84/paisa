// Package manifest persists import provenance and row records beneath the
// configured paisa config directory.
//
// Layout:
//
//	<root>/imports/<sourceType>/<importId>/manifest.json
//	<root>/imports/<sourceType>/<importId>/records.json
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// RecordTypeImportManifest identifies manifest documents.
const RecordTypeImportManifest = "import_manifest"

// ParserInfo describes the parser that produced an import.
type ParserInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// RowCounts summarizes how many rows a parser saw and accepted.
type RowCounts struct {
	Read     int `json:"read"`
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
	Skipped  int `json:"skipped"`
}

// ImportError describes a rejected row or a file-level parse failure.
type ImportError struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	RecordLocator string `json:"record_locator,omitempty"`
}

// Manifest is the provenance record for one imported source file.
type Manifest struct {
	RecordType    string        `json:"record_type"`
	ImportID      string        `json:"import_id"`
	SourceType    string        `json:"source_type"`
	OriginalPath  string        `json:"original_path"`
	ContentSHA256 string        `json:"content_sha256"`
	ImportedAt    string        `json:"imported_at"`
	Status        string        `json:"status"`
	Parser        ParserInfo    `json:"parser"`
	RowCounts     RowCounts     `json:"row_counts"`
	Errors        []ImportError `json:"errors"`
}

var importIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// ImportID derives the stable identifier for an imported source. It is the
// SHA-256 of source_type, original_path and content_sha256 separated by
// newlines.
func ImportID(sourceType, originalPath, contentSHA256 string) string {
	payload := sourceType + "\n" + originalPath + "\n" + contentSHA256
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// Write atomically stores the manifest and the associated records. It returns
// the path of the written manifest. Files are written with mode 0600 and
// directories are created with mode 0700.
func Write(root string, m Manifest, records any) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("import root is required")
	}
	if err := validateSegment("source_type", m.SourceType); err != nil {
		return "", err
	}
	if !importIDPattern.MatchString(m.ImportID) {
		return "", fmt.Errorf("import_id must be a 64 character SHA-256 hex string")
	}
	if m.RecordType == "" {
		m.RecordType = RecordTypeImportManifest
	}

	manifestData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal import manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')

	recordsData, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal import records: %w", err)
	}
	recordsData = append(recordsData, '\n')

	dir := filepath.Join(root, "imports", m.SourceType, m.ImportID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create import directory: %w", err)
	}

	// Write records first so a manifest never references records that do not
	// exist yet.
	if err := writeFileAtomically(filepath.Join(dir, "records.json"), recordsData); err != nil {
		return "", fmt.Errorf("write import records: %w", err)
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	if err := writeFileAtomically(manifestPath, manifestData); err != nil {
		return "", fmt.Errorf("write import manifest: %w", err)
	}

	return manifestPath, nil
}

// ReadLatest returns the newest manifest for each original path within a
// source type. A missing source type yields an empty history.
func ReadLatest(root, sourceType string) ([]Manifest, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("import root is required")
	}
	if err := validateSegment("source_type", sourceType); err != nil {
		return nil, err
	}

	base := filepath.Join(root, "imports", sourceType)
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return []Manifest{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read import directory: %w", err)
	}

	latest := make(map[string]Manifest)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(base, entry.Name(), "manifest.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read import manifest %s: %w", entry.Name(), err)
		}

		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse import manifest %s: %w", entry.Name(), err)
		}
		if m.OriginalPath == "" {
			continue
		}

		previous, seen := latest[m.OriginalPath]
		if !seen || importedAtAfter(m.ImportedAt, previous.ImportedAt) {
			latest[m.OriginalPath] = m
		}
	}

	result := make([]Manifest, 0, len(latest))
	for _, m := range latest {
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].OriginalPath == result[j].OriginalPath {
			return result[i].ImportedAt > result[j].ImportedAt
		}
		return result[i].OriginalPath < result[j].OriginalPath
	})

	return result, nil
}

// VerifySource ensures the manifest's original path stays inside root and that
// the source file still has the recorded SHA-256.
func VerifySource(root string, m Manifest) error {
	path, err := resolveInsideRoot(root, m.OriginalPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(m.ContentSHA256) == "" {
		return fmt.Errorf("manifest has no content hash")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read import source: %w", err)
	}

	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, strings.TrimSpace(m.ContentSHA256)) {
		return fmt.Errorf("import source hash mismatch for %q", m.OriginalPath)
	}

	return nil
}

func writeFileAtomically(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		if temp != nil {
			_ = temp.Close()
		}
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err = temp.Write(data); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Chmod(0600); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	temp = nil

	if err = os.Rename(tempPath, path); err != nil {
		return err
	}
	return nil
}

func validateSegment(field, value string) error {
	if value == "" || value == "." || value == ".." {
		return fmt.Errorf("%s is invalid", field)
	}
	if filepath.Base(value) != value || strings.ContainsAny(value, `/\`) || strings.Contains(value, ":") {
		return fmt.Errorf("%s must be a single path segment", field)
	}
	return nil
}

func resolveInsideRoot(root, relativePath string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("import root is required")
	}
	if strings.TrimSpace(relativePath) == "" {
		return "", fmt.Errorf("original_path is required")
	}
	if filepath.IsAbs(relativePath) || filepath.VolumeName(relativePath) != "" {
		return "", fmt.Errorf("path %q is outside import root", relativePath)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve import root: %w", err)
	}
	candidate, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(relativePath)))
	if err != nil {
		return "", fmt.Errorf("resolve import path: %w", err)
	}

	if !isInside(rootAbs, candidate) {
		return "", fmt.Errorf("path %q is outside import root", relativePath)
	}

	// Resolve symlinks when they exist so a link inside the root cannot point
	// at a file outside it.
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err == nil {
		realCandidate, err := filepath.EvalSymlinks(candidate)
		if err == nil && !isInside(realRoot, realCandidate) {
			return "", fmt.Errorf("path %q is outside import root", relativePath)
		}
	}

	return candidate, nil
}

func isInside(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func importedAtAfter(candidate, current string) bool {
	candidateTime, candidateErr := time.Parse(time.RFC3339Nano, candidate)
	currentTime, currentErr := time.Parse(time.RFC3339Nano, current)
	if candidateErr == nil && currentErr == nil {
		return candidateTime.After(currentTime)
	}
	return candidate > current
}
