package utils

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNowUsesConfiguredTimeZone(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(
		"journal_path: test.ledger\ndb_path: test.db\ntime_zone: Europe/Berlin\n",
	), ""))
	now = time.Time{}
	t.Cleanup(func() { now = time.Time{} })

	assert.Equal(t, "Europe/Berlin", Now().Location().String())
}

func TestSetNowUsesUTC(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(
		"journal_path: test.ledger\ndb_path: test.db\ntime_zone: Europe/Berlin\n",
	), ""))
	now = time.Time{}
	t.Cleanup(func() { now = time.Time{} })

	SetNow("2022-02-07")
	assert.Equal(t, "2022-02-07T00:00:00Z", Now().UTC().Format(time.RFC3339))
}

func TestOpenDBUsesWALAndBusyTimeout(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.LoadConfig(fmt.Appendf(nil,
		"journal_path: %s/main.ledger\ndb_path: %s/test.db\n", dir, dir,
	), ""))
	db, err := OpenDB()
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	var mode string
	require.NoError(t, db.Raw("PRAGMA journal_mode").Scan(&mode).Error)
	assert.Equal(t, "wal", mode)
	var timeout int
	require.NoError(t, db.Raw("PRAGMA busy_timeout").Scan(&timeout).Error)
	assert.Equal(t, 5000, timeout)
}

func TestOpenDBAppliesPragmasToEveryConnection(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.LoadConfig(fmt.Appendf(nil,
		"journal_path: %s/main.ledger\ndb_path: %s/test.db\n", dir, dir,
	), ""))
	db, err := OpenDB()
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(4)

	ctx := context.Background()
	first, err := sqlDB.Conn(ctx)
	require.NoError(t, err)
	defer first.Close()

	second, err := sqlDB.Conn(ctx)
	require.NoError(t, err)
	defer second.Close()

	for name, conn := range map[string]*sql.Conn{"first": first, "second": second} {
		var timeout int
		require.NoError(t, conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout), name)
		assert.Equal(t, 5000, timeout, name)
	}
}

func TestFinancialYearHelpersUseConfiguredStartingMonth(t *testing.T) {
	require.NoError(t, config.LoadConfig([]byte(
		"journal_path: test.ledger\ndb_path: test.db\nfinancial_year_starting_month: 7\n",
	), ""))

	june := time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)
	july := time.Date(2024, time.July, 1, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, "2023-24", FY(june))
	assert.Equal(t, "2023 - 24", FYHuman(june))
	assert.Equal(t, "2024-25", FY(july))
	assert.Equal(t, "2023-07-01", BeginningOfFinancialYear(june).Format("2006-01-02"))
	assert.Equal(t, "2024-07-01", BeginningOfFinancialYear(july).Format("2006-01-02"))

	start, _ := ParseFY("2024")
	assert.Equal(t, "2024-07-01", start.Format("2006-01-02"))
}

func TestBuildSubPath(t *testing.T) {
	baseDirectory := filepath.FromSlash("/usr/home/john/paisa")

	path, err := BuildSubPath(baseDirectory, "main.ledger")
	assert.Nil(t, err)
	assert.Equal(t, filepath.Join(baseDirectory, "main.ledger"), path)

	path, err = BuildSubPath(baseDirectory, "subfolder/main.ledger")
	assert.Nil(t, err)
	assert.Equal(t, filepath.Join(baseDirectory, "subfolder", "main.ledger"), path)

	path, err = BuildSubPath(baseDirectory, "../../../subfolder/travel.ledger")
	assert.Error(t, err)

	path, err = BuildSubPath(baseDirectory, "..")
	assert.Error(t, err)

	path, err = BuildSubPath(baseDirectory, "./..")
	assert.Error(t, err)

	path, err = BuildSubPath(baseDirectory, "./../test.ledger")
	assert.Error(t, err)
}
