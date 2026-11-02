package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeJournalFile(t *testing.T, root, name, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestJournalFilesIncludesNestedAndQuotedFiles(t *testing.T) {
	root := t.TempDir()
	main := writeJournalFile(t, root, "main.ledger", "include sub/a.ledger\ninclude \"sub/b.ledger\"\n")
	a := writeJournalFile(t, root, "sub/a.ledger", "2024-01-01 a\n    Assets:A    1\n    Income:B   -1\n")
	b := writeJournalFile(t, root, "sub/b.ledger", "include c.ledger\n")
	c := writeJournalFile(t, root, "sub/c.ledger", "2024-01-02 c\n    Assets:A    1\n    Income:B   -1\n")

	for _, cli := range []Ledger{LedgerCLI{}, HLedgerCLI{}} {
		files, err := cli.Files(main)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{main, a, b, c}, files)
	}
}

func TestJournalFilesExpandsGlobs(t *testing.T) {
	root := t.TempDir()
	main := writeJournalFile(t, root, "main.ledger", "include accounts/*.ledger\n")
	first := writeJournalFile(t, root, "accounts/one.ledger", "")
	second := writeJournalFile(t, root, "accounts/two.ledger", "")

	files, err := LedgerCLI{}.Files(main)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{main, first, second}, files)
}

func TestJournalFilesBreaksIncludeCycles(t *testing.T) {
	root := t.TempDir()
	main := writeJournalFile(t, root, "main.ledger", "include other.ledger\n")
	other := writeJournalFile(t, root, "other.ledger", "include main.ledger\n")

	files, err := HLedgerCLI{}.Files(main)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{main, other}, files)
}

func TestJournalFilesWithoutIncludesReturnsJournal(t *testing.T) {
	main := writeJournalFile(t, t.TempDir(), "main.ledger", "2024-01-01 a\n    Assets:A    1\n    Income:B   -1\n")

	files, err := LedgerCLI{}.Files(main)
	require.NoError(t, err)
	require.Equal(t, []string{main}, files)
}
