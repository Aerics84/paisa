package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var passwordLinePattern = regexp.MustCompile(`(?im)^(\s*(?:-\s*)?password\s*:\s*)(?:"[^"]*"|'[^']*'|[^,\r\n}]+)`)

// GenerateBackupZip creates a zip archive containing the journal files,
// configuration file (paisa.yaml), and sheet files.
//
// The database cache file is never included. Password values in the
// configuration are redacted unless includeSecrets is true. Files that resolve
// outside the configuration or journal directory are skipped. A journal file
// that cannot be read makes the export fail instead of producing an incomplete
// backup.
func GenerateBackupZip(includeSecrets bool) ([]byte, string, error) {
	return generateBackupZip(includeSecrets, ledger.Cli().Files)
}

func generateBackupZip(includeSecrets bool, listJournalFiles func(string) ([]string, error)) ([]byte, string, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	configDir := config.GetConfigDir()
	journalPath := config.GetJournalPath()
	journalDir := filepath.Dir(journalPath)

	configRoot := resolveBackupPath(configDir)
	journalRoot := resolveBackupPath(journalDir)
	dbPath := resolveBackupPath(config.GetDBPath())

	isPathSafe := func(target string) bool {
		if target == "" {
			return false
		}
		resolved := resolveBackupPath(target)
		return isPathWithin(configRoot, resolved) || isPathWithin(journalRoot, resolved)
	}

	isDatabaseFile := func(target string) bool {
		resolved := resolveBackupPath(target)
		if resolved == dbPath {
			return true
		}

		base := strings.ToLower(filepath.Base(resolved))
		dbBase := strings.ToLower(filepath.Base(dbPath))
		if dbBase != "" {
			if base == dbBase || strings.HasPrefix(base, dbBase+"-") {
				return true
			}
		}

		switch strings.ToLower(filepath.Ext(base)) {
		case ".db", ".sqlite", ".sqlite3":
			return true
		default:
			return false
		}
	}

	addFileToZip := func(zipPath string, content []byte, modTime time.Time) error {
		header := &zip.FileHeader{
			Name:     filepath.ToSlash(zipPath),
			Method:   zip.Deflate,
			Modified: modTime,
		}
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = writer.Write(content)
		return err
	}

	// 1. Add journal files.
	journalFiles, err := listJournalFiles(journalPath)
	if err != nil || len(journalFiles) == 0 {
		journalFiles = []string{journalPath}
	}

	addedSources := make(map[string]bool)
	usedZipPaths := make(map[string]bool)
	for _, file := range journalFiles {
		if file == "" {
			continue
		}

		if !filepath.IsAbs(file) {
			if _, statErr := os.Stat(file); statErr != nil {
				if candidate := filepath.Join(journalDir, file); fileExists(candidate) {
					file = candidate
				}
			}
		}

		resolved := resolveBackupPath(file)
		if addedSources[resolved] {
			continue
		}
		addedSources[resolved] = true

		if !isPathSafe(file) {
			log.WithField("path", file).Warn("Skipping file outside allowed directory during backup")
			continue
		}
		if isDatabaseFile(file) {
			log.WithField("path", file).Debug("Skipping database file during backup")
			continue
		}

		info, err := os.Stat(file)
		if err != nil {
			return nil, "", fmt.Errorf("failed to stat journal file %s: %w", file, err)
		}
		if info.IsDir() {
			return nil, "", fmt.Errorf("journal path is a directory: %s", file)
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read journal file %s: %w", file, err)
		}

		relPath, err := filepath.Rel(journalDir, file)
		if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
			relPath = filepath.Base(file)
		}

		zipPath := uniqueBackupZipPath("journals/"+filepath.ToSlash(relPath), usedZipPaths)
		if err := addFileToZip(zipPath, content, info.ModTime()); err != nil {
			return nil, "", fmt.Errorf("failed to add journal file %s: %w", file, err)
		}
	}

	// 2. Add the configuration file.
	configPath := config.GetConfigPath()
	if configPath != "" && isPathSafe(configPath) && !isDatabaseFile(configPath) {
		if content, err := os.ReadFile(configPath); err == nil {
			modTime := time.Now()
			if info, err := os.Stat(configPath); err == nil {
				modTime = info.ModTime()
			}

			if !includeSecrets {
				content = redactConfigSecrets(content)
			}

			if err := addFileToZip("config/paisa.yaml", content, modTime); err != nil {
				return nil, "", fmt.Errorf("failed to add config file: %w", err)
			}
		}
	}

	// 3. Add sheet files.
	sheetDir := config.GetSheetDir()
	if sheetDir != "" && isPathSafe(sheetDir) {
		_ = filepath.WalkDir(sheetDir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if !isPathSafe(path) || isDatabaseFile(path) {
				return nil
			}

			if !strings.EqualFold(filepath.Ext(entry.Name()), EXTENSION) {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			modTime := time.Now()
			if info, err := entry.Info(); err == nil {
				modTime = info.ModTime()
			}

			relPath, err := filepath.Rel(sheetDir, path)
			if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || filepath.IsAbs(relPath) {
				relPath = filepath.Base(path)
			}

			zipPath := uniqueBackupZipPath("sheets/"+filepath.ToSlash(relPath), usedZipPaths)
			if err := addFileToZip(zipPath, content, modTime); err != nil {
				log.WithError(err).Warn("Failed to add sheet file to backup")
			}
			return nil
		})
	}

	if err := zipWriter.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize backup zip: %w", err)
	}

	filename := fmt.Sprintf("paisa-backup-%s.zip", time.Now().Format("2006-01-02"))
	return buf.Bytes(), filename, nil
}

// HandleExportBackup handles GET /api/backup/export.
func HandleExportBackup(c *gin.Context) {
	includeSecrets := c.Query("include_secrets") == "true"

	log.WithFields(log.Fields{
		"event":           "backup_export",
		"include_secrets": includeSecrets,
		"ip":              c.ClientIP(),
	}).Info("Exporting backup archive")

	zipData, filename, err := GenerateBackupZip(includeSecrets)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/zip", zipData)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func resolveBackupPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

func isPathWithin(root string, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func uniqueBackupZipPath(path string, used map[string]bool) string {
	if !used[path] {
		used[path] = true
		return path
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

func redactConfigSecrets(content []byte) []byte {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return redactConfigSecretsByLine(content)
	}

	redactYAMLNode(&root)
	stripYAMLAnchorsAndAliases(&root)

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		_ = encoder.Close()
		return redactConfigSecretsByLine(content)
	}
	if err := encoder.Close(); err != nil {
		return redactConfigSecretsByLine(content)
	}

	return buf.Bytes()
}

func redactYAMLNode(node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			redactYAMLNode(child)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if strings.EqualFold(key.Value, "password") {
				redactYAMLValue(node.Content[i+1])
				node.Content[i+1] = redactedYAMLValue()
				continue
			}
			redactYAMLNode(node.Content[i+1])
		}
	}
}

// redactYAMLValue replaces the given value with a redacted scalar. When the
// value is an alias, the anchored node it points to is redacted as well so the
// secret does not leak through the anchor declaration.
func redactYAMLValue(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		redactYAMLValue(node.Alias)
		return
	}

	node.Kind = yaml.ScalarNode
	node.Tag = "!!str"
	node.Value = "[REDACTED]"
	node.Style = yaml.DoubleQuotedStyle
	node.Content = nil
	node.Alias = nil
	node.Anchor = ""
}

func redactedYAMLValue() *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: "[REDACTED]",
		Style: yaml.DoubleQuotedStyle,
	}
}

// stripYAMLAnchorsAndAliases removes anchors and resolves aliases by cloning
// their targets. This guarantees that no value hidden behind an anchor can
// survive redaction in the emitted YAML.
func stripYAMLAnchorsAndAliases(node *yaml.Node) {
	if node == nil {
		return
	}
	node.Anchor = ""

	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for i, child := range node.Content {
			if child != nil && child.Kind == yaml.AliasNode && child.Alias != nil {
				node.Content[i] = cloneYAMLNode(child.Alias, make(map[*yaml.Node]bool))
				continue
			}
			stripYAMLAnchorsAndAliases(child)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if key != nil && key.Kind == yaml.AliasNode && key.Alias != nil {
				node.Content[i] = cloneYAMLNode(key.Alias, make(map[*yaml.Node]bool))
			} else {
				stripYAMLAnchorsAndAliases(key)
			}

			value := node.Content[i+1]
			if value != nil && value.Kind == yaml.AliasNode && value.Alias != nil {
				node.Content[i+1] = cloneYAMLNode(value.Alias, make(map[*yaml.Node]bool))
				continue
			}
			stripYAMLAnchorsAndAliases(value)
		}
	}
}

func cloneYAMLNode(node *yaml.Node, stack map[*yaml.Node]bool) *yaml.Node {
	if node == nil {
		return nil
	}
	if stack[node] {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	}
	stack[node] = true
	defer delete(stack, node)

	clone := *node
	clone.Anchor = ""
	clone.Alias = nil
	clone.Content = nil
	for _, child := range node.Content {
		if child != nil && child.Kind == yaml.AliasNode && child.Alias != nil {
			clone.Content = append(clone.Content, cloneYAMLNode(child.Alias, stack))
			continue
		}
		clone.Content = append(clone.Content, cloneYAMLNode(child, stack))
	}

	return &clone
}

func redactConfigSecretsByLine(content []byte) []byte {
	return passwordLinePattern.ReplaceAll(content, []byte(`${1}"[REDACTED]"`))
}
