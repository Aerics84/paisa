package server

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"os"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/ananthakumaran/paisa/internal/ledger"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

type LedgerFile struct {
	Name      string   `json:"name"`
	Content   string   `json:"content"`
	Versions  []string `json:"versions"`
	Operation string   `json:"operation"`
}

func GetFiles(deps Deps) EditorFilesResponse {
	db := deps.DB
	var accounts []string
	var payees []string
	var commodities []string
	db.Model(&posting.Posting{}).Distinct().Order("account").Pluck("Account", &accounts)
	db.Model(&posting.Posting{}).Distinct().Order("payee").Pluck("Payee", &payees)
	db.Model(&posting.Posting{}).Distinct().Order("commodity").Pluck("Commodity", &commodities)

	path := deps.Config.JournalPath()

	files := []LedgerFile{}
	dir := filepath.Dir(path)
	paths, _ := doublestar.FilepathGlob(dir + "/**/*" + filepath.Ext(path))

	for _, path = range paths {
		files = append(files, *readLedgerFileWithVersions(dir, path))
	}

	return EditorFilesResponse{
		Files:       files,
		Accounts:    nonNilSlice(accounts),
		Payees:      nonNilSlice(payees),
		Commodities: nonNilSlice(commodities),
	}
}

func GetFile(file LedgerFile, store *config.Store) EditorFileResponse {
	path := store.JournalPath()
	dir := filepath.Dir(path)
	filePath, err := utils.BuildSubPath(dir, file.Name)
	if err != nil {
		return EditorFileResponse{Message: "Invalid file name"}
	}
	return EditorFileResponse{File: readLedgerFile(dir, filePath)}
}

// pruneOldBackups keeps every backup from the cutoff day, the newest backup
// per calendar day for the previous seven days, and the newest backup per
// seven-day bucket for the four weeks before that. Backup names embed a
// sortable timestamp.
func pruneOldBackups(filePath string, cutoff time.Time) {
	backups := utils.Glob(filePath, ".backup.*")
	prefix := filepath.Base(filePath) + ".backup."
	keep := map[string]bool{}
	latestByDay := map[string]string{}
	latestByBucket := map[int]string{}

	for _, backup := range backups {
		name := filepath.Base(backup)
		if !strings.HasPrefix(name, prefix) {
			keep[backup] = true
			continue
		}

		suffix := strings.TrimPrefix(name, prefix)
		parsed, err := time.ParseInLocation("2006-01-02-15-04-05.000", suffix, cutoff.Location())
		if err != nil {
			keep[backup] = true
			continue
		}

		offset := int(calendarDay(cutoff).Sub(calendarDay(parsed)).Hours() / 24)
		switch {
		case offset <= 0:
			keep[backup] = true
		case offset <= 7:
			day := parsed.Format("2006-01-02")
			if current, ok := latestByDay[day]; !ok || backup > current {
				latestByDay[day] = backup
			}
		case offset <= 35:
			bucket := (offset - 8) / 7
			if current, ok := latestByBucket[bucket]; !ok || backup > current {
				latestByBucket[bucket] = backup
			}
		}
	}

	for _, backup := range latestByDay {
		keep[backup] = true
	}
	for _, backup := range latestByBucket {
		keep[backup] = true
	}

	for _, backup := range backups {
		if keep[backup] {
			continue
		}
		if err := os.Remove(backup); err != nil {
			log.Warn(err)
		}
	}
}

func calendarDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func DeleteBackups(file LedgerFile, store *config.Store) EditorFileResponse {
	path := store.JournalPath()
	dir := filepath.Dir(path)
	filePath, err := utils.BuildSubPath(dir, file.Name)
	if err != nil {
		return EditorFileResponse{Message: "Invalid file name"}
	}

	if !store.Get().Readonly {
		versions := utils.Glob(filePath, ".backup.*")
		for _, version := range versions {
			err := os.Remove(version)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	return EditorFileResponse{File: readLedgerFileWithVersions(dir, filePath)}
}

func SaveFile(deps Deps, file LedgerFile) EditorSaveResponse {
	path := deps.Config.JournalPath()
	if extension := filepath.Ext(path); extension != "" && filepath.Ext(file.Name) != extension {
		return EditorSaveResponse{Errors: []FileError{}, Saved: false, Message: "File name must end with " + extension}
	}

	errors, _, err := validateFile(file, deps.Config)
	if err != nil {
		return EditorSaveResponse{Errors: fileErrors(errors), Saved: false, Message: "Validation failed"}
	}

	dir := filepath.Dir(path)

	filePath, err := utils.BuildSubPath(dir, file.Name)
	if err != nil {
		log.Warn(err)
		return EditorSaveResponse{Errors: fileErrors(errors), Saved: false, Message: "Invalid file name"}
	}

	now := utils.Now()
	backupPath := filePath + ".backup." + now.Format("2006-01-02-15-04-05.000")

	err = os.MkdirAll(filepath.Dir(filePath), 0700)
	if err != nil {
		log.Warn(err)
		return EditorSaveResponse{Errors: fileErrors(errors), Saved: false, Message: "Failed to create directory"}
	}

	fileStat, err := os.Stat(filePath)
	if err != nil && file.Operation != "overwrite" && file.Operation != "create" {
		log.Warn(err)
		return EditorSaveResponse{Errors: fileErrors(errors), Saved: false, Message: "File does not exist"}
	}

	var perm os.FileMode = 0644
	if err == nil {
		if file.Operation == "create" {
			return EditorSaveResponse{Errors: fileErrors(errors), Saved: false, Message: "File already exists"}
		}

		perm = fileStat.Mode().Perm()
		existingContent, err := os.ReadFile(filePath)
		if err != nil {
			log.Warn(err)
			return EditorSaveResponse{Errors: fileErrors(errors), Saved: false, Message: "Failed to read file"}
		}

		err = os.WriteFile(backupPath, existingContent, perm)
		if err != nil {
			log.Warn(err)
			return EditorSaveResponse{Errors: fileErrors(errors), Saved: false, Message: "Failed to create backup"}
		}
	}

	err = os.WriteFile(filePath, []byte(file.Content), perm)
	if err != nil {
		log.Warn(err)
		return EditorSaveResponse{Errors: fileErrors(errors), Saved: false, Message: "Failed to write file"}
	}

	pruneOldBackups(filePath, now)

	Sync(deps, SyncRequest{Journal: true})

	return EditorSaveResponse{Errors: fileErrors(errors), Saved: true, File: readLedgerFileWithVersions(dir, filePath)}
}

func ValidateFile(file LedgerFile, store *config.Store) EditorValidateResponse {
	errors, output, _ := validateFile(file, store)
	return EditorValidateResponse{Errors: fileErrors(errors), Output: output}
}

func validateFile(file LedgerFile, store *config.Store) ([]ledger.LedgerFileError, string, error) {
	path := store.JournalPath()
	if file.Name != "" {
		filePath, err := utils.BuildSubPath(filepath.Dir(path), file.Name)
		if err != nil || filePath == filepath.Clean(filepath.Dir(path)) {
			return []ledger.LedgerFileError{{LineFrom: 1, LineTo: 1, Message: "Invalid file name"}}, "", os.ErrInvalid
		}
		path = filePath
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return []ledger.LedgerFileError{{LineFrom: 1, LineTo: 1, Message: err.Error()}}, "", err
	}

	tmpfile, err := os.CreateTemp(filepath.Dir(path), "paisa-tmp-")
	if err != nil {
		return []ledger.LedgerFileError{{LineFrom: 1, LineTo: 1, Message: err.Error()}}, "", err
	}

	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(file.Content)); err != nil {
		return []ledger.LedgerFileError{{LineFrom: 1, LineTo: 1, Message: err.Error()}}, "", err
	}

	if err := tmpfile.Close(); err != nil {
		return []ledger.LedgerFileError{{LineFrom: 1, LineTo: 1, Message: err.Error()}}, "", err
	}

	return ledger.Cli().ValidateFile(tmpfile.Name())
}

func readLedgerFile(dir string, path string) *LedgerFile {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	name, err := filepath.Rel(dir, path)

	return &LedgerFile{
		Name:     name,
		Content:  string(content),
		Versions: []string{},
	}
}

func readLedgerFileWithVersions(dir string, path string) *LedgerFile {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	versions := utils.Glob(path, ".backup.*")
	versionPaths := lo.Map(versions, func(path string, _ int) string {
		name, err := filepath.Rel(dir, path)
		if err != nil {
			log.Fatal(err)
		}

		return name
	})
	sort.Sort(sort.Reverse(sort.StringSlice(versionPaths)))

	name, err := filepath.Rel(dir, path)
	if err != nil {
		log.Fatal(err)
	}

	return &LedgerFile{
		Name:     name,
		Content:  string(content),
		Versions: versionPaths,
	}
}
