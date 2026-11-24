package migrations

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "paisa.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db, path
}

func userVersion(t *testing.T, db *gorm.DB) int {
	t.Helper()

	var version int
	require.NoError(t, db.Raw("PRAGMA user_version").Scan(&version).Error)
	return version
}

// withExtraMigration appends a migration to the global list for the duration
// of the test.
func withExtraMigration(t *testing.T, migration Migration) {
	t.Helper()

	original := migrations
	migrations = append(append([]Migration{}, original...), migration)
	t.Cleanup(func() { migrations = original })
}

func TestMigrateIsIdempotent(t *testing.T) {
	db, path := openTestDB(t)

	require.NoError(t, Migrate(db, path))
	require.NoError(t, Migrate(db, path))

	require.Equal(t, 1, userVersion(t, db))
	require.True(t, db.Migrator().HasTable("postings"))
	require.True(t, db.Migrator().HasTable("prices"))
}

func TestFailedMigrationRollsBack(t *testing.T) {
	db, path := openTestDB(t)
	require.NoError(t, Migrate(db, path))
	require.Equal(t, 1, userVersion(t, db))

	withExtraMigration(t, Migration{
		Version: 2,
		Name:    "broken",
		Up: func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE TABLE half_migrated (id integer)").Error; err != nil {
				return err
			}
			return errors.New("boom")
		},
	})

	require.Error(t, Migrate(db, path))
	require.Equal(t, 1, userVersion(t, db))
	require.False(t, db.Migrator().HasTable("half_migrated"))
}

func TestMigrateRefusesNewerDatabase(t *testing.T) {
	db, path := openTestDB(t)
	require.NoError(t, db.Exec("PRAGMA user_version = 99").Error)

	require.ErrorIs(t, Migrate(db, path), ErrDatabaseNewer)
	require.Equal(t, 99, userVersion(t, db))
	require.False(t, db.Migrator().HasTable("postings"))
}

func TestDestructiveMigrationCreatesBackup(t *testing.T) {
	db, path := openTestDB(t)
	require.NoError(t, Migrate(db, path))

	withExtraMigration(t, Migration{
		Version:     2,
		Name:        "destructive-test",
		Destructive: true,
		Up:          func(*gorm.DB) error { return nil },
	})

	require.NoError(t, Migrate(db, path))
	require.Equal(t, 2, userVersion(t, db))

	matches, err := filepath.Glob(path + ".backup-v2-*")
	require.NoError(t, err)
	require.Len(t, matches, 1)

	backup, err := os.ReadFile(matches[0])
	require.NoError(t, err)
	require.Greater(t, len(backup), 0)
	require.Equal(t, []byte("SQLite format 3\x00"), backup[:16])
}

func TestMigrateExistingV08Database(t *testing.T) {
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

	var postings int64
	require.NoError(t, db.Table("postings").Count(&postings).Error)
	require.Equal(t, int64(54), postings)

	var prices int64
	require.NoError(t, db.Table("prices").Count(&prices).Error)
	require.Equal(t, int64(4), prices)
}
