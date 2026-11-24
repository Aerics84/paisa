// Package migrations applies versioned schema migrations to the Paisa
// SQLite database.
package migrations

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ananthakumaran/paisa/internal/model/cache"
	"github.com/ananthakumaran/paisa/internal/model/cii"
	"github.com/ananthakumaran/paisa/internal/model/import_rule"
	mutualfundModel "github.com/ananthakumaran/paisa/internal/model/mutualfund/scheme"
	npsModel "github.com/ananthakumaran/paisa/internal/model/nps/scheme"
	"github.com/ananthakumaran/paisa/internal/model/portfolio"
	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/model/price"
	"gorm.io/gorm"
)

// Migration describes a single schema change. Version numbers must be
// positive and strictly increasing across the list.
type Migration struct {
	Version     int
	Name        string
	Destructive bool
	Up          func(*gorm.DB) error
}

// ErrDatabaseNewer is returned when the database was written by a newer
// Paisa version that uses a schema this build does not know about.
var ErrDatabaseNewer = errors.New("database was created by a newer Paisa version")

// migrations is the ordered list of known migrations. Tests may temporarily
// append to it.
var migrations = []Migration{
	{
		Version: 1,
		Name:    "baseline",
		Up:      baselineUp,
	},
}

// baselineUp brings a database up to the v1 schema. It reuses GORM's
// AutoMigrate so existing databases from before user_version was tracked are
// upgraded in place without losing any rows.
func baselineUp(db *gorm.DB) error {
	return db.AutoMigrate(
		&npsModel.Scheme{},
		&mutualfundModel.Scheme{},
		&posting.Posting{},
		&price.Price{},
		&portfolio.Portfolio{},
		&cii.CII{},
		&import_rule.Rule{},
		&cache.Cache{},
	)
}

// Migrate applies every migration that is newer than the database's
// user_version. Each migration and its version bump run in a single
// transaction, so a failure leaves the database at the last successful
// version.
func Migrate(db *gorm.DB, dbPath string) error {
	current, err := readVersion(db)
	if err != nil {
		return err
	}

	latest, err := latestVersion()
	if err != nil {
		return err
	}

	if current > latest {
		return ErrDatabaseNewer
	}

	for _, migration := range migrations {
		if migration.Version <= current {
			continue
		}

		if migration.Destructive {
			if err := backup(dbPath, migration.Version); err != nil {
				return fmt.Errorf("backup database before migration %d (%s): %w", migration.Version, migration.Name, err)
			}
		}

		if err := apply(db, migration); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", migration.Version, migration.Name, err)
		}

		current = migration.Version
	}

	return nil
}

func readVersion(db *gorm.DB) (int, error) {
	var version int
	if err := db.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		return 0, fmt.Errorf("read database version: %w", err)
	}
	return version, nil
}

func latestVersion() (int, error) {
	latest := 0
	for _, migration := range migrations {
		if migration.Version <= 0 {
			return 0, fmt.Errorf("invalid migration version %d (%s)", migration.Version, migration.Name)
		}
		if migration.Version <= latest {
			return 0, fmt.Errorf("migration versions are not strictly increasing at %d (%s)", migration.Version, migration.Name)
		}
		latest = migration.Version
	}
	return latest, nil
}

func apply(db *gorm.DB, migration Migration) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := migration.Up(tx); err != nil {
			return err
		}
		return tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", migration.Version)).Error
	})
}

// backup copies the database file before a destructive migration runs. The
// copy is synced to disk and closed before the migration starts.
func backup(dbPath string, version int) error {
	timestamp := time.Now().UTC().Format("20060102T150405.000000000")
	return copyFile(dbPath, fmt.Sprintf("%s.backup-v%d-%s", dbPath, version, timestamp))
}

func copyFile(source string, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		_ = os.Remove(target)
		return err
	}

	return nil
}
