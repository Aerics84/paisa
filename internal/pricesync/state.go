package pricesync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// stateSchemaVersion is the version of the persisted state format. It is
	// bumped whenever the on-disk format changes in an incompatible way so
	// that migrations can be added later.
	stateSchemaVersion = 1

	// stateFileSuffix is appended to the database path to build the path of
	// the persisted state file.
	stateFileSuffix = ".price-update.json"

	// stateTempPattern is the pattern of the temporary file used for the
	// atomic write. The file is created in the same directory as the state
	// file so that the rename stays atomic.
	stateTempPattern = ".price-update-*.tmp"
)

// State describes the outcome of the last price update run. It is persisted
// as JSON next to the database file so that the server and the CLI share the
// state of the automatic price update.
type State struct {
	// SchemaVersion is the version of the persisted format.
	SchemaVersion int
	// LastAttemptAt is the time the last run was started.
	LastAttemptAt time.Time
	// LastSuccessAt is the time of the last fully successful run.
	LastSuccessAt time.Time
	// LastPartialSuccessAt is the time of the last partially successful run.
	LastPartialSuccessAt time.Time
	// LastError holds the error message of the last failed run.
	LastError string
	// LastWarning holds the warnings of the last run.
	LastWarning string
	// FailedCommodities lists the commodities that failed in the last run.
	FailedCommodities []string
	// LastDurationMS is the duration of the last run in milliseconds.
	LastDurationMS int64
	// LastFull is true when the last run was a full update.
	LastFull bool
	// LastJobID is the id of the last started job.
	LastJobID uint64
	// LastJobSuccess is true when the last job finished successfully.
	LastJobSuccess bool
}

// stateFile is the on-disk representation of State. Timestamps are stored as
// RFC3339 strings and are empty when the timestamp is unset.
type stateFile struct {
	SchemaVersion        int      `json:"schema_version"`
	LastAttemptAt        string   `json:"last_attempt_at"`
	LastSuccessAt        string   `json:"last_success_at"`
	LastPartialSuccessAt string   `json:"last_partial_success_at"`
	LastError            string   `json:"last_error"`
	LastWarning          string   `json:"last_warning"`
	FailedCommodities    []string `json:"failed_commodities"`
	LastDurationMS       int64    `json:"last_duration_ms"`
	LastFull             bool     `json:"last_full"`
	LastJobID            uint64   `json:"last_job_id"`
	LastJobSuccess       bool     `json:"last_job_success"`
}

// MarshalJSON writes the state in the documented on-disk format.
func (s State) MarshalJSON() ([]byte, error) {
	return json.Marshal(stateFile{
		SchemaVersion:        s.SchemaVersion,
		LastAttemptAt:        formatStateTime(s.LastAttemptAt),
		LastSuccessAt:        formatStateTime(s.LastSuccessAt),
		LastPartialSuccessAt: formatStateTime(s.LastPartialSuccessAt),
		LastError:            s.LastError,
		LastWarning:          s.LastWarning,
		FailedCommodities:    s.FailedCommodities,
		LastDurationMS:       s.LastDurationMS,
		LastFull:             s.LastFull,
		LastJobID:            s.LastJobID,
		LastJobSuccess:       s.LastJobSuccess,
	})
}

// UnmarshalJSON reads the state from the documented on-disk format.
func (s *State) UnmarshalJSON(data []byte) error {
	var file stateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	lastAttemptAt, err := parseStateTime(file.LastAttemptAt)
	if err != nil {
		return fmt.Errorf("last_attempt_at: %w", err)
	}
	lastSuccessAt, err := parseStateTime(file.LastSuccessAt)
	if err != nil {
		return fmt.Errorf("last_success_at: %w", err)
	}
	lastPartialSuccessAt, err := parseStateTime(file.LastPartialSuccessAt)
	if err != nil {
		return fmt.Errorf("last_partial_success_at: %w", err)
	}

	*s = State{
		SchemaVersion:        file.SchemaVersion,
		LastAttemptAt:        lastAttemptAt,
		LastSuccessAt:        lastSuccessAt,
		LastPartialSuccessAt: lastPartialSuccessAt,
		LastError:            file.LastError,
		LastWarning:          file.LastWarning,
		FailedCommodities:    file.FailedCommodities,
		LastDurationMS:       file.LastDurationMS,
		LastFull:             file.LastFull,
		LastJobID:            file.LastJobID,
		LastJobSuccess:       file.LastJobSuccess,
	}

	return nil
}

// StateStore persists the state of the price update.
type StateStore interface {
	// Load returns the persisted state. A missing state file is not an
	// error, it returns the zero state instead.
	Load() (State, error)
	// Save persists the given state.
	Save(State) error
}

// FileStateStore persists the state in a JSON file. The file is written
// atomically: the data goes to a temporary file in the same directory which
// is then renamed over the state file.
type FileStateStore struct {
	path string
}

// NewFileStateStore returns a store that persists the state in the given file.
func NewFileStateStore(path string) *FileStateStore {
	return &FileStateStore{path: path}
}

// Load reads the state file. A missing file returns the zero state without
// an error, a corrupt file returns the zero state and an error.
func (s *FileStateStore) Load() (State, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		log.Errorf("Failed to read price update state %s: %v", s.path, err)
		return State{}, fmt.Errorf("read price update state %s: %w", s.path, err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		log.Errorf("Failed to decode price update state %s: %v", s.path, err)
		return State{}, fmt.Errorf("decode price update state %s: %w", s.path, err)
	}

	if state.SchemaVersion == 0 {
		state.SchemaVersion = stateSchemaVersion
	}

	return state, nil
}

// Save writes the state file. The state is written to a temporary file in the
// same directory which is flushed and then renamed over the state file, so a
// crash never leaves a half written file behind. Errors are returned and
// logged, they never abort the caller.
func (s *FileStateStore) Save(state State) error {
	state.SchemaVersion = stateSchemaVersion

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Errorf("Failed to encode price update state: %v", err)
		return fmt.Errorf("encode price update state: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, stateTempPattern)
	if err != nil {
		log.Errorf("Failed to create temporary price update state file in %s: %v", dir, err)
		return fmt.Errorf("create temporary price update state file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		discardTempFile(tmp, tmpPath)
		log.Errorf("Failed to write price update state %s: %v", tmpPath, err)
		return fmt.Errorf("write price update state: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		discardTempFile(tmp, tmpPath)
		log.Errorf("Failed to sync price update state %s: %v", tmpPath, err)
		return fmt.Errorf("sync price update state: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		log.Errorf("Failed to close price update state %s: %v", tmpPath, err)
		return fmt.Errorf("close price update state: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		log.Errorf("Failed to replace price update state %s: %v", s.path, err)
		return fmt.Errorf("replace price update state %s: %w", s.path, err)
	}

	return nil
}

// discardTempFile closes and removes a temporary state file that could not be
// written completely.
func discardTempFile(tmp *os.File, path string) {
	tmp.Close()
	os.Remove(path)
}

// MemoryStateStore keeps the state in memory. It is used by tests and by
// callers that do not want to persist anything.
type MemoryStateStore struct {
	mu    sync.Mutex
	state State
	saved bool
}

// NewMemoryStateStore returns a store that keeps the state in memory.
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{}
}

// Load returns the stored state. As long as nothing was saved the zero state
// is returned.
func (s *MemoryStateStore) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.saved {
		return State{}, nil
	}

	return s.state.clone(), nil
}

// Save stores the state in memory.
func (s *MemoryStateStore) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state.SchemaVersion = stateSchemaVersion
	s.state = state.clone()
	s.saved = true

	return nil
}

// StatePathForDB returns the path of the state file for the given database
// path. The state file lives next to the database file.
func StatePathForDB(dbPath string) string {
	return dbPath + stateFileSuffix
}

// clone returns a copy of the state so that callers cannot reach the stored
// state through the returned slice.
func (s State) clone() State {
	out := s
	if s.FailedCommodities != nil {
		out.FailedCommodities = make([]string, len(s.FailedCommodities))
		copy(out.FailedCommodities, s.FailedCommodities)
	}

	return out
}

// formatStateTime renders a timestamp for the state file. Unset timestamps
// are stored as an empty string.
func formatStateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Format(time.RFC3339Nano)
}

// parseStateTime reads a timestamp from the state file. An empty string is
// the unset timestamp.
func parseStateTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	return time.Parse(time.RFC3339, value)
}

// Compile time checks that both stores implement StateStore.
var (
	_ StateStore = (*FileStateStore)(nil)
	_ StateStore = (*MemoryStateStore)(nil)
)
