package cache

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	log "github.com/sirupsen/logrus"
)

// The server keeps derived data in memory (see Clear). Another process, such as
// `paisa update` run from cron or systemd, can change the database without the
// server knowing. To close that gap, every process that finishes writing data
// calls MarkSynced, which writes a marker file next to the database. The server
// calls ClearIfStale on each API request and drops its in-memory caches when the
// marker changed.

var epochPath = func() string {
	return config.GetDBPath() + ".synced"
}

var (
	epochMu   sync.Mutex
	epochSeen int64
	epochInit bool
)

// MarkSynced records that the database was updated. Call it after a sync
// finishes, whether it ran in the server or in the command line tool.
func MarkSynced() {
	err := os.WriteFile(epochPath(), []byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0o644)
	if err != nil {
		log.Warnf("could not write sync marker: %v", err)
	}
}

// ClearIfStale clears the in-memory caches when another sync finished since the
// last call. A missing marker counts as epoch zero, so the first marker that
// appears later still triggers a clear.
func ClearIfStale() {
	var epoch int64
	if info, err := os.Stat(epochPath()); err == nil {
		epoch = info.ModTime().UnixNano()
	}

	epochMu.Lock()
	defer epochMu.Unlock()

	if !epochInit {
		epochInit = true
		epochSeen = epoch
		return
	}

	if epoch != epochSeen {
		epochSeen = epoch
		Clear()
	}
}
