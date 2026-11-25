// Package migrations applies versioned schema migrations to the Paisa
// SQLite database.
package migrations

import (
	"errors"
	"fmt"
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
		return fmt.Errorf("%w: database %s has user_version %d, latest supported is %d", ErrDatabaseNewer, dbPath, current, latest)
	}

	for _, migration := range migrations {
		if migration.Version <= current {
			continue
		}

		if migration.Destructive {
			if err := backup(db, dbPath, migration.Version); err != nil {
				return fmt.Errorf("backup database %s before migration %d (%s): %w", dbPath, migration.Version, migration.Name, err)
			}
		}

		if err := apply(db, migration); err != nil {
			return fmt.Errorf("migrate database %s: migration %d (%s) failed: %w", dbPath, migration.Version, migration.Name, err)
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

// backup creates a consistent copy of the database before a destructive
// migration runs.
//
// VACUUM INTO is used instead of a file copy: SQLite writes the target from a
// single read snapshot, so committed WAL frames are included and the result is
// transactionally consistent. It reads through the live connection, so a
// concurrent writer (for example a running `paisa serve`) can never rewrite
// the main database file in the middle of the copy and produce a torn backup.
// A writer that SQLite cannot accommodate makes VACUUM INTO return an error
// instead of a partial snapshot, and Migrate then aborts before the
// destructive migration starts. VACUUM INTO cannot run inside a transaction,
// so it is called before apply opens one; SQLite also refuses to overwrite an
// existing target file.
func backup(db *gorm.DB, dbPath string, version int) error {
	if dbPath == "" {
		return errors.New("database path is required to back up the database")
	}

	timestamp := time.Now().UTC().Format("20060102T150405.000000000")
	target := fmt.Sprintf("%s.backup-v%d-%s", dbPath, version, timestamp)

	if err := db.Exec("VACUUM INTO ?", target).Error; err != nil {
		return fmt.Errorf("create database backup %s: %w", target, err)
	}

	return nil
}
