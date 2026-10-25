package utils

import (
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
