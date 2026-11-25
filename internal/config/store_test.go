package config

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const storeMinimalConfig = "journal_path: main.ledger\ndb_path: paisa.db\n"

func newStoreWithFile(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "paisa.yaml")
	store := NewStore()
	require.NoError(t, store.Load([]byte(storeMinimalConfig), path))
	return store, path
}

func TestStoreReloadIsConsistent(t *testing.T) {
	store := NewStore()
	pathA := filepath.Join(t.TempDir(), "a.yaml")
	pathB := filepath.Join(t.TempDir(), "b.yaml")
	require.NoError(t, store.Load([]byte("journal_path: a.ledger\ndb_path: a.db\n"), pathA))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = store.Get()
		}()
		go func() {
			defer wg.Done()
			_ = store.Load([]byte("journal_path: b.ledger\ndb_path: b.db\n"), pathB)
		}()
	}
	wg.Wait()

	path := store.Get().JournalPath
	require.Contains(t, []string{"a.ledger", "b.ledger"}, path)
}

func TestGetReturnsIsolatedCopy(t *testing.T) {
	store := NewStore()
	require.NoError(t, store.Load([]byte(`
journal_path: main.ledger
db_path: paisa.db
commodities:
  - name: VWCE
    type: mutualfund
    price: { provider: com-yahoo, code: VWCE }
`), filepath.Join(t.TempDir(), "paisa.yaml")))

	first := store.Get()
	first.Commodities[0].Name = "MUTATED"

	second := store.Get()
	require.Equal(t, "VWCE", second.Commodities[0].Name)
}

func TestCloneIsolatesNestedPointersAndSlices(t *testing.T) {
	store := NewStore()
	require.NoError(t, store.Load([]byte(`
journal_path: main.ledger
db_path: paisa.db
salary_income_accounts: [Income:Salary]
tax_deductions:
  accounts:
    - account: Expenses:Health
      category: health
`), filepath.Join(t.TempDir(), "paisa.yaml")))

	cfg := store.Get()
	require.NotNil(t, cfg.SalaryIncomeAccounts)
	(*cfg.SalaryIncomeAccounts)[0] = "Income:Mutated"
	cfg.TaxDeductions.Accounts = nil

	again := store.Get()
	require.Equal(t, []string{"Income:Salary"}, *again.SalaryIncomeAccounts)
	require.NotNil(t, again.TaxDeductions.Accounts)
	require.Len(t, *again.TaxDeductions.Accounts, 1)
}

func TestUpdateIsAtomicAndValidated(t *testing.T) {
	store, _ := newStoreWithFile(t)

	require.Error(t, store.Update(func(c *Config) error {
		c.JournalPath = "changed.ledger"
		return errors.New("reject")
	}))
	require.Equal(t, "main.ledger", store.Get().JournalPath)

	require.NoError(t, store.Update(func(c *Config) error {
		c.JournalPath = "changed.ledger"
		return nil
	}))
	require.Equal(t, "changed.ledger", store.Get().JournalPath)
}

func TestUpdateRejectsInvalidConfigAndKeepsPreviousSnapshot(t *testing.T) {
	store, _ := newStoreWithFile(t)

	require.Error(t, store.Update(func(c *Config) error {
		c.LedgerCli = "not-a-ledger"
		return nil
	}))
	require.Equal(t, "ledger", store.Get().LedgerCli)

	require.Error(t, store.Update(func(c *Config) error {
		c.TimeZone = "Not/AZone"
		return nil
	}))
	require.Equal(t, "", store.Get().TimeZone)
	require.Equal(t, "main.ledger", store.Get().JournalPath)
}

func TestUpdateWritesConfigAndUpdatesTimeZone(t *testing.T) {
	store, path := newStoreWithFile(t)

	require.NoError(t, store.Update(func(c *Config) error {
		c.TimeZone = "Europe/Berlin"
		c.DefaultCurrency = "EUR"
		return nil
	}))

	assert.Equal(t, "Europe/Berlin", store.TimeZone().String())
	assert.Equal(t, "EUR", store.Get().DefaultCurrency)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "time_zone: Europe/Berlin")
	assert.Contains(t, string(content), "default_currency: EUR")
}

func TestUpdateRejectsReadonlyConfig(t *testing.T) {
	store := NewStore()
	require.NoError(t, store.Load([]byte(storeMinimalConfig+"readonly: true\n"), filepath.Join(t.TempDir(), "paisa.yaml")))

	err := store.Update(func(c *Config) error {
		c.JournalPath = "changed.ledger"
		return nil
	})
	require.ErrorIs(t, err, ErrReadonly)
	require.Equal(t, "main.ledger", store.Get().JournalPath)
}

func TestStorePathAndConfigDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "paisa.yaml")
	store := NewStore()
	require.NoError(t, store.Load([]byte(storeMinimalConfig), path))

	assert.Equal(t, path, store.Path())
	assert.Equal(t, filepath.Dir(path), store.ConfigDir())
}

func TestStorePathAccessorsResolveRelativeToConfigDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paisa.yaml")
	store := NewStore()
	require.NoError(t, store.Load([]byte(`
journal_path: journal/main.ledger
db_path: data/paisa.db
sheets_directory: sheets
`), path))

	assert.Equal(t, filepath.Join(dir, "journal", "main.ledger"), store.JournalPath())
	assert.Equal(t, filepath.Join(dir, "data", "paisa.db"), store.DBPath())
	assert.Equal(t, filepath.Join(dir, "sheets"), store.SheetDir())
}

func TestStorePathAccessorsKeepAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "absolute.ledger")
	db := filepath.Join(dir, "absolute.db")
	store := NewStore()
	require.NoError(t, store.Load([]byte(
		"journal_path: "+filepath.ToSlash(journal)+"\ndb_path: "+filepath.ToSlash(db)+"\n",
	), filepath.Join(dir, "paisa.yaml")))

	assert.Equal(t, filepath.Clean(journal), filepath.Clean(store.JournalPath()))
	assert.Equal(t, filepath.Clean(db), filepath.Clean(store.DBPath()))
}

func TestStoreSaveWritesSnapshot(t *testing.T) {
	store, path := newStoreWithFile(t)

	require.NoError(t, store.Save([]byte("journal_path: saved.ledger\ndb_path: paisa.db\n")))

	assert.Equal(t, "saved.ledger", store.Get().JournalPath)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "journal_path: saved.ledger")
}

func TestStoreTaxHelpersUseOwnSnapshot(t *testing.T) {
	indian := NewStore()
	require.NoError(t, indian.Load([]byte("journal_path: main.ledger\ndb_path: paisa.db\ntax_regime: india\ntax_deductions:\n  accounts:\n    - account: Expenses:Health\n      category: health\n"), ""))
	german := NewStore()
	require.NoError(t, german.Load([]byte("journal_path: main.ledger\ndb_path: paisa.db\ntax_regime: germany\n"), ""))

	assert.True(t, indian.SupportsIndiaTaxFeatures())
	assert.True(t, indian.SupportsScheduleAL())
	assert.False(t, indian.SupportsGermanyTaxFeatures())
	assert.Len(t, indian.TaxDeductionAccounts(), 1)
	assert.Equal(t, "Expenses:Health", indian.TaxDeductionAccounts()[0].Account)

	assert.True(t, german.SupportsGermanyTaxFeatures())
	assert.False(t, german.SupportsIndiaTaxFeatures())
	assert.False(t, german.SupportsScheduleAL())
	assert.Equal(t, defaultTaxDeductionAccounts, german.TaxDeductionAccounts())
}

func TestLoadFailureKeepsPreviousSnapshot(t *testing.T) {
	store, path := newStoreWithFile(t)

	err := store.Load([]byte("journal_path: [unterminated\n"), path)
	require.Error(t, err)
	assert.Equal(t, "main.ledger", store.Get().JournalPath)

	err = store.Load([]byte("journal_path: other.ledger\ndb_path: other.db\ntax_deductions:\n  accounts:\n    - account: Expenses:Health\n      category: made_up\n"), path)
	require.Error(t, err)
	assert.Equal(t, "main.ledger", store.Get().JournalPath)
}

func TestLoadFileReadsUnderSequenceLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paisa.yaml")
	require.NoError(t, os.WriteFile(path, []byte("journal_path: old.ledger\ndb_path: paisa.db\n"), 0644))

	store := NewStore()
	require.NoError(t, store.Load([]byte(storeMinimalConfig), path))

	store.updateMu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- store.LoadFile(path)
	}()

	select {
	case err := <-done:
		store.updateMu.Unlock()
		t.Fatalf("LoadFile completed while the sequence lock was held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	// Rewriting the file while the sequence lock is held proves that the read
	// happens after the lock is acquired, not before it.
	require.NoError(t, os.WriteFile(path, []byte("journal_path: new.ledger\ndb_path: paisa.db\n"), 0644))
	store.updateMu.Unlock()

	require.NoError(t, <-done)
	assert.Equal(t, "new.ledger", store.Get().JournalPath)
}

func TestUpdateWaitsForSequenceLock(t *testing.T) {
	store, _ := newStoreWithFile(t)

	store.updateMu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- store.Update(func(c *Config) error {
			c.JournalPath = "changed.ledger"
			return nil
		})
	}()

	select {
	case err := <-done:
		store.updateMu.Unlock()
		t.Fatalf("Update completed while the sequence lock was held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	store.updateMu.Unlock()

	require.NoError(t, <-done)
	assert.Equal(t, "changed.ledger", store.Get().JournalPath)
}

func TestSaveConfigWaitsForSequenceLock(t *testing.T) {
	store, path := newStoreWithFile(t)

	store.updateMu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- store.saveContent([]byte("journal_path: saved.ledger\ndb_path: paisa.db\n"))
	}()

	select {
	case err := <-done:
		store.updateMu.Unlock()
		t.Fatalf("saveContent completed while the sequence lock was held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	store.updateMu.Unlock()

	require.NoError(t, <-done)
	_, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, "saved.ledger", store.Get().JournalPath)
}

func TestFinancialYearStartingMonthAccessor(t *testing.T) {
	store := NewStore()
	require.NoError(t, store.Load([]byte("journal_path: main.ledger\ndb_path: paisa.db\nfinancial_year_starting_month: 7\n"), ""))
	assert.Equal(t, time.July, store.FinancialYearStartingMonth())

	require.NoError(t, store.Load([]byte(storeMinimalConfig), ""))
	assert.Equal(t, time.April, store.FinancialYearStartingMonth(), "the india profile default must still apply")
}

func TestInitializationTransitions(t *testing.T) {
	store := NewStore()
	require.False(t, store.Initialized())
	require.False(t, store.AutoCreated())

	require.NoError(t, store.Load([]byte(storeMinimalConfig), ""))
	require.True(t, store.Initialized())
	require.False(t, store.AutoCreated())

	store.MarkAutoCreated()
	require.True(t, store.AutoCreated())
	require.False(t, store.Initialized())
}

func TestTryBeginOnboardingSucceedsOnce(t *testing.T) {
	store := NewStore()
	require.False(t, store.TryBeginOnboarding(), "a store without an auto-created config must not start onboarding")

	require.NoError(t, store.Load([]byte(storeMinimalConfig), ""))
	store.MarkAutoCreated()

	require.True(t, store.TryBeginOnboarding())
	require.False(t, store.TryBeginOnboarding(), "the onboarding lock must be exclusive")

	store.AbortOnboarding()
	require.True(t, store.TryBeginOnboarding(), "aborting must release the lock")

	store.MarkInitialized()
	require.True(t, store.Initialized())
	require.False(t, store.AutoCreated())
	require.False(t, store.TryBeginOnboarding(), "an initialized config must never start onboarding again")
}

// TestAbortOnboardingResetsInitializedForAutoCreatedConfig covers a failed
// first-run attempt: the generated configuration was loaded (initialized),
// then rolled back. Aborting must make the installation look unconfigured
// again so the user can retry with a different profile or mode.
func TestAbortOnboardingResetsInitializedForAutoCreatedConfig(t *testing.T) {
	store := NewStore()
	require.NoError(t, store.Load([]byte(storeMinimalConfig), ""))
	store.MarkAutoCreated()
	require.True(t, store.TryBeginOnboarding())

	// Simulate the failed attempt loading the generated file.
	require.NoError(t, store.Load([]byte(storeMinimalConfig), ""))
	require.True(t, store.Initialized())

	store.AbortOnboarding()
	require.False(t, store.Initialized())
	require.True(t, store.AutoCreated())
	require.True(t, store.TryBeginOnboarding(), "a retry must be possible")

	store.MarkInitialized()
	require.True(t, store.Initialized())
	require.False(t, store.AutoCreated())
}

// TestAbortOnboardingKeepsInitializedForUserConfig ensures aborting does not
// forget an already initialized configuration that was never auto-created.
func TestAbortOnboardingKeepsInitializedForUserConfig(t *testing.T) {
	store := NewStore()
	require.NoError(t, store.Load([]byte(storeMinimalConfig), ""))

	store.AbortOnboarding()

	require.True(t, store.Initialized())
	require.False(t, store.AutoCreated())
}

func TestConcurrentTryBeginOnboardingOnlyOneWinner(t *testing.T) {
	store := NewStore()
	require.NoError(t, store.Load([]byte(storeMinimalConfig), ""))
	store.MarkAutoCreated()

	var winners int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if store.TryBeginOnboarding() {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, winners)
}

func TestDefaultStoreIsShared(t *testing.T) {
	require.NotNil(t, DefaultStore())
	assert.Same(t, defaultStore, DefaultStore())
}
