package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fileAPI struct {
	name, extension string
	get, delete     func(string) gin.H
}

var fileAPIs = []fileAPI{
	{"ledger", ".ledger",
		func(name string) gin.H { return GetFile(LedgerFile{Name: name}) },
		func(name string) gin.H { return DeleteBackups(LedgerFile{Name: name}) }},
	{"sheet", ".paisa",
		func(name string) gin.H { return GetSheet(SheetFile{Name: name}) },
		func(name string) gin.H { return DeleteSheetBackups(SheetFile{Name: name}) }},
}

func configureEditor(t *testing.T, root string, readonly bool) {
	t.Helper()
	content, err := json.Marshal(map[string]interface{}{
		"journal_path":     filepath.Join(root, "main.ledger"),
		"sheets_directory": root,
		"db_path":          filepath.Join(root, "paisa.db"),
		"readonly":         readonly,
	})
	require.NoError(t, err)
	require.NoError(t, config.LoadConfig(content, ""))
}

func writeEditorFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
}

func TestEditorRejectsTraversal(t *testing.T) {
	for _, api := range fileAPIs {
		t.Run(api.name, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "journal")
			require.NoError(t, os.Mkdir(base, 0700))
			outside := filepath.Join(root, "secret"+api.extension)
			writeEditorFile(t, outside, "secret")
			writeEditorFile(t, outside+".backup.1", "backup")
			for _, readonly := range []bool{false, true} {
				configureEditor(t, base, readonly)
				for _, name := range []string{"../secret" + api.extension, "nested/../../secret" + api.extension} {
					for _, handler := range []func(string) gin.H{api.get, api.delete} {
						require.Equal(t, "Invalid file name", handler(filepath.FromSlash(name))["message"])
					}
				}
			}
			content, err := os.ReadFile(outside)
			require.NoError(t, err)
			require.Equal(t, "secret", string(content))
			require.FileExists(t, outside+".backup.1")
		})
	}
}

func TestDeleteBackupsDoesNotExpandFilename(t *testing.T) {
	for _, api := range fileAPIs {
		t.Run(api.name, func(t *testing.T) {
			base := t.TempDir()
			configureEditor(t, base, false)
			name := "main[1]" + api.extension
			path := filepath.Join(base, name)
			writeEditorFile(t, path, "current")
			writeEditorFile(t, path+".backup.1", "previous")
			unrelated := filepath.Join(base, "main1"+api.extension+".backup.1")
			writeEditorFile(t, unrelated, "unrelated")

			api.delete(name)

			require.NoFileExists(t, path+".backup.1")
			require.FileExists(t, unrelated)
		})
	}
}

func TestPruneOldBackupsKeepsTodayAndBucketsOlderDays(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "main.ledger")
	cutoff := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		stamp string
		keep  bool
	}{
		{"2026-10-31-08-00-00.000", true},      // today: keep all
		{"2026-10-31-11-00-00.000", true},      // today: keep all
		{"2026-10-30-08-00-00.000", false},     // n=1: older same-day copy
		{"2026-10-30-12-00-00.000", true},      // n=1: newest of the day
		{"2026-10-24-08-00-00.000", false},     // n=7: older same-day copy
		{"2026-10-24-12-00-00.000", true},      // n=7: newest of the day
		{"2026-10-23-08-00-00.000", false},     // n=8: older copy in bucket 0
		{"2026-10-23-12-00-00.000", true},      // n=8: newest in bucket 0
		{"2026-10-17-12-00-00.000", false},     // n=14: older copy in bucket 0
		{"2026-10-16-08-00-00.000", false},     // n=15: older copy in bucket 1
		{"2026-10-16-12-00-00.000", true},      // n=15: newest in bucket 1
		{"2026-10-10-12-00-00.000", false},     // n=21: older copy in bucket 1
		{"2026-10-09-08-00-00.000", false},     // n=22: older copy in bucket 2
		{"2026-10-09-12-00-00.000", true},      // n=22: newest in bucket 2
		{"2026-10-03-12-00-00.000", false},     // n=28: older copy in bucket 2
		{"2026-10-02-08-00-00.000", false},     // n=29: older copy in bucket 3
		{"2026-10-02-12-00-00.000", true},      // n=29: newest in bucket 3
		{"2026-09-26-12-00-00.000", false},     // n=35: older copy in bucket 3
		{"2026-09-25-12-00-00.000", false},     // n=36: older than 35 days
		{"2020-01-01-00-00-00.000", false},     // old but fully valid timestamp
		{"2020-01-01-invalid-timestamp", true}, // malformed suffix: keep
		{"manual", true},                       // unparseable: keep
	}

	expected := []string{}
	for _, tc := range cases {
		path := journal + ".backup." + tc.stamp
		require.NoError(t, os.WriteFile(path, []byte("x"), 0644))
		if tc.keep {
			expected = append(expected, path)
		}
	}

	pruneOldBackups(journal, cutoff)

	matches, err := filepath.Glob(journal + ".backup.*")
	require.NoError(t, err)
	require.ElementsMatch(t, expected, matches)
}

func TestPruneOldBackupsUsesCalendarDatesAcrossTimeZones(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "main.ledger")
	loc := time.FixedZone("CET", 3600)
	cutoff := time.Date(2026, 10, 31, 23, 30, 0, 0, loc)

	write := func(stamp string) string {
		path := journal + ".backup." + stamp
		require.NoError(t, os.WriteFile(path, []byte("x"), 0644))
		return path
	}

	day7 := write("2026-10-24-12-00-00.000")      // calendar day 7
	day8Early := write("2026-10-23-00-05-00.000") // calendar day 8, ~8d23h before cutoff
	day8Late := write("2026-10-23-23-55-00.000")  // calendar day 8, ~7d23h before cutoff
	day14 := write("2026-10-17-12-00-00.000")     // calendar day 14, older in bucket 0

	pruneOldBackups(journal, cutoff)

	require.FileExists(t, day7)
	require.NoFileExists(t, day8Early)
	require.FileExists(t, day8Late)
	require.NoFileExists(t, day14)
}

func TestPruneOldBackupsHandlesDSTCalendarBoundaries(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("Europe/Berlin time zone data is unavailable")
	}
	dir := t.TempDir()
	journal := filepath.Join(dir, "main.ledger")
	cutoff := time.Date(2026, 10, 27, 0, 30, 0, 0, loc) // CET, after the DST change

	write := func(stamp string) string {
		path := journal + ".backup." + stamp
		require.NoError(t, os.WriteFile(path, []byte("x"), 0644))
		return path
	}

	day3 := write("2026-10-24-00-30-00.000") // CEST, calendar day 3
	day8 := write("2026-10-19-23-30-00.000") // CEST, calendar day 8
	day9 := write("2026-10-18-23-30-00.000") // CEST, same bucket, older

	pruneOldBackups(journal, cutoff)

	require.FileExists(t, day3)
	require.FileExists(t, day8)
	require.NoFileExists(t, day9)
}

func TestPruneOldBackupsDoesNotTouchOtherFiles(t *testing.T) {
	dir := t.TempDir()
	cutoff := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	journal := filepath.Join(dir, "main.ledger")
	other := filepath.Join(dir, "other.ledger")

	oldJournal := journal + ".backup.2026-01-01-00-00-00.000"
	oldOther := other + ".backup.2026-01-01-00-00-00.000"
	require.NoError(t, os.WriteFile(oldJournal, []byte("x"), 0644))
	require.NoError(t, os.WriteFile(oldOther, []byte("x"), 0644))

	pruneOldBackups(journal, cutoff)

	require.NoFileExists(t, oldJournal)
	require.FileExists(t, oldOther)
}
