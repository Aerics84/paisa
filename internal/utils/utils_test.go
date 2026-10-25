package utils

import (
	"fmt"
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

func TestOpenDBUsesWALAndBusyTimeout(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.LoadConfig([]byte(fmt.Sprintf(
		"journal_path: %s/main.ledger\ndb_path: %s/test.db\n", dir, dir,
	)), ""))
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

func TestBuildSubPath(t *testing.T) {
	path, err := BuildSubPath("/usr/home/john/paisa", "main.ledger")
	assert.Nil(t, err)
	assert.Equal(t, "/usr/home/john/paisa/main.ledger", path)

	path, err = BuildSubPath("/usr/home/john/paisa", "subfolder/main.ledger")
	assert.Nil(t, err)
	assert.Equal(t, "/usr/home/john/paisa/subfolder/main.ledger", path)

	path, err = BuildSubPath("/usr/home/john/paisa", "../../../subfolder/travel.ledger")
	assert.Error(t, err)

	path, err = BuildSubPath("/usr/home/john/paisa", "..")
	assert.Error(t, err)

	path, err = BuildSubPath("/usr/home/john/paisa", "./..")
	assert.Error(t, err)

	path, err = BuildSubPath("/usr/home/john/paisa", "./../test.ledger")
	assert.Error(t, err)
}
