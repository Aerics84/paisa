package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestMigratedV086DatabaseRemainsReadable documents the supported downgrade
// path: a 0.8.6.6 database migrated to the current schema must still answer the
// queries the older binary issues. The schema change is additive, so the old
// binary keeps reading the database and simply ignores new columns.
func TestMigratedV086DatabaseRemainsReadable(t *testing.T) {
	source, err := os.ReadFile("testdata/v0.8.6.db")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "paisa.db")
	require.NoError(t, os.WriteFile(path, source, 0o600))

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	require.Equal(t, 0, userVersion(t, db))

	require.NoError(t, Migrate(db, path))
	require.Equal(t, 1, userVersion(t, db))

	// Queries that 0.8.6.6 already issued must keep working after the upgrade.
	var postings int64
	require.NoError(t, db.Table("postings").Count(&postings).Error)
	require.Equal(t, int64(54), postings)

	var prices int64
	require.NoError(t, db.Table("prices").Count(&prices).Error)
	require.Equal(t, int64(4), prices)

	// Reading a row through the old column set must still succeed.
	var payee string
	require.NoError(t, db.Table("postings").Select("payee").Limit(1).Scan(&payee).Error)
	require.NotEmpty(t, payee)

	var integrity string
	require.NoError(t, db.Raw("PRAGMA integrity_check").Scan(&integrity).Error)
	require.Equal(t, "ok", integrity)
}
