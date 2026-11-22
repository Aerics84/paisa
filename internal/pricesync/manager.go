package pricesync

import (
	"context"
	"sync"
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// cooldownInterval is the minimum time between two automatic runs.
const cooldownInterval = 30 * time.Minute

// StartMode describes how a run was started.
type StartMode int

const (
	// StartManual ignores the cooldown and stale checks.
	StartManual StartMode = iota
	// StartAutomatic only runs when the schedule says it should.
	StartAutomatic
)

// Status is a snapshot of the manager's state at a point in time.
type Status struct {
	Enabled              bool       `json:"enabled"`
	Managed              bool       `json:"managed"`
	Running              bool       `json:"running"`
	JobID                uint64     `json:"job_id"`
	LastJobID            uint64     `json:"last_job_id"`
	LastJobSuccess       bool       `json:"last_job_success"`
	Stale                bool       `json:"stale"`
	ShouldUpdate         bool       `json:"should_update"`
	LastAttemptAt        *time.Time `json:"last_attempt_at"`
	LastSuccessAt        *time.Time `json:"last_success_at"`
	LastPartialSuccessAt *time.Time `json:"last_partial_success_at"`
	NextRunAt            *time.Time `json:"next_run_at"`
	LastError            string     `json:"last_error"`
	LastWarning          string     `json:"last_warning"`
	FailedCommodities    []string   `json:"failed_commodities"`
}

// Manager coordinates the automatic price update runs. It enforces a single
// price run at a time, tracks the persisted run state and drives the scheduler.
type Manager struct {
	mu sync.Mutex
	// db is retained for callers that wire the manager to a connection; the
	// actual syncing is delegated to syncFn.
	db     *gorm.DB
	cfg    func() config.Config
	store  StateStore
	syncFn func(SyncOptions) Outcome
	runner *Runner
	now    func() time.Time

	state     State
	jobID     uint64
	running   bool
	startedAt time.Time

	wake   chan struct{}
	cancel context.CancelFunc
	once   sync.Once
}

// New returns a Manager that loads its initial state from store. A load error
// is logged and the manager starts from the zero state; it is never fatal.
func New(db *gorm.DB, cfg func() config.Config, store StateStore, syncFn func(SyncOptions) Outcome, runner *Runner, now func() time.Time) *Manager {
	if cfg == nil {
		cfg = config.GetConfig
	}
	if runner == nil {
		runner = DefaultRunner
	}
	if now == nil {
		now = time.Now
	}

	state, err := store.Load()
	if err != nil {
		log.Errorf("Failed to load price update state: %v", err)
		state = State{}
	}
	state.SchemaVersion = stateSchemaVersion

	return &Manager{
		db:     db,
		cfg:    cfg,
		store:  store,
		syncFn: syncFn,
		runner: runner,
		now:    now,
		state:  state,
		jobID:  state.LastJobID,
		wake:   make(chan struct{}, 1),
	}
}

// schedule derives the Schedule from the current configuration.
func (m *Manager) schedule() Schedule {
	cfg := m.cfg()
	return ParseSchedule(cfg.EffectivePriceUpdate(), m.location(cfg))
}

// location resolves the configured time zone, falling back to the global one.
func (m *Manager) location(cfg config.Config) *time.Location {
	if cfg.TimeZone != "" {
		if loc, err := time.LoadLocation(cfg.TimeZone); err == nil {
			return loc
		}
	}
	return config.TimeZone()
}

// Status returns a snapshot of the manager's state as of now.
func (m *Manager) Status(now time.Time) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked(now)
}

func (m *Manager) statusLocked(now time.Time) Status {
	cfg := m.cfg()
	schedule := m.schedule()

	enabled := schedule.Active && !cfg.Readonly
	managed := hasManagedCommodity(cfg)
	running := m.running || m.runner.Busy()

	slot, ok := schedule.MostRecentSlot(now)
	stale := m.state.LastSuccessAt.IsZero() || (ok && m.state.LastSuccessAt.Before(slot))

	cooldown := m.state.LastAttemptAt.IsZero() || now.Sub(m.state.LastAttemptAt) >= cooldownInterval
	should := enabled && managed && stale && !running && cooldown

	status := Status{
		Enabled:              enabled,
		Managed:              managed,
		Running:              running,
		LastJobID:            m.state.LastJobID,
		LastJobSuccess:       m.state.LastJobSuccess,
		Stale:                stale,
		ShouldUpdate:         should,
		LastAttemptAt:        timePtr(m.state.LastAttemptAt),
		LastSuccessAt:        timePtr(m.state.LastSuccessAt),
		LastPartialSuccessAt: timePtr(m.state.LastPartialSuccessAt),
		LastError:            m.state.LastError,
		LastWarning:          m.state.LastWarning,
		FailedCommodities:    m.state.FailedCommodities,
	}

	if running {
		status.JobID = m.jobID
	}

	if enabled {
		if next, ok := schedule.NextSlot(now); ok {
			status.NextRunAt = &next
		}
	}

	return status
}

// Start launches a price run unless one is already running. Automatic starts
// additionally require the schedule to say the state is stale and past the
// cooldown; manual starts ignore those checks. It returns the current status
// and whether a new job was launched.
func (m *Manager) Start(opts SyncOptions, mode StartMode) (Status, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	status := m.statusLocked(now)

	if m.running || m.runner.Busy() {
		return status, false
	}

	if mode == StartAutomatic && !status.ShouldUpdate {
		return status, false
	}

	if !m.runner.TryStart() {
		return status, false
	}

	m.jobID++
	m.running = true
	m.startedAt = now

	m.state.LastAttemptAt = now
	m.state.LastJobID = m.jobID
	m.state.LastJobSuccess = false
	m.state.LastFull = opts.Full
	if err := m.store.Save(m.state); err != nil {
		log.Errorf("Failed to persist price update state: %v", err)
	}

	go m.run(opts, m.jobID)

	return m.statusLocked(now), true
}

// run executes the sync function and records the outcome in the persisted
// state. The runner slot is released via a deferred Finish so that a panic in
// the sync function never leaks the runner.
func (m *Manager) run(opts SyncOptions, jobID uint64) {
	defer m.runner.Finish()

	outcome := m.syncFn(opts)

	m.mu.Lock()
	m.running = false

	now := m.now()
	m.state.LastError = outcome.Error
	m.state.LastWarning = outcome.Warning
	m.state.FailedCommodities = outcome.FailedCommodities
	m.state.LastDurationMS = now.Sub(m.startedAt).Milliseconds()
	m.state.LastJobSuccess = outcome.Success

	switch {
	case outcome.Success:
		m.state.LastSuccessAt = now
		m.state.LastPartialSuccessAt = time.Time{}
	case outcome.Partial:
		m.state.LastPartialSuccessAt = now
	}

	saveErr := m.store.Save(m.state)
	m.mu.Unlock()

	if saveErr != nil {
		log.Errorf("Failed to persist price update state: %v", saveErr)
	}

	switch {
	case outcome.Success:
		log.Infof("Price update job %d succeeded", jobID)
	case outcome.Partial:
		log.Warnf("Price update job %d partially failed: %s", jobID, outcome.Error)
	default:
		log.Errorf("Price update job %d failed: %s", jobID, outcome.Error)
	}
}

// StartScheduler starts the background scheduler. It is idempotent: only the
// first call launches the scheduler goroutine.
func (m *Manager) StartScheduler(ctx context.Context) {
	m.once.Do(func() {
		ctx, cancel := context.WithCancel(ctx)
		m.mu.Lock()
		m.cancel = cancel
		m.mu.Unlock()
		go m.schedulerLoop(ctx)
	})
}

func (m *Manager) schedulerLoop(ctx context.Context) {
	log.Info("Price update scheduler started")
	defer log.Info("Price update scheduler stopped")

	for {
		if ctx.Err() != nil {
			return
		}

		status := m.Status(m.now())
		if status.ShouldUpdate && ctx.Err() == nil {
			m.Start(SyncOptions{Full: false}, StartAutomatic)
		}

		next, ok := m.schedule().NextSlot(m.now())
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-m.wake:
				continue
			}
		}

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-m.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

// Wake re-evaluates the schedule immediately without blocking.
func (m *Manager) Wake() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Stop cancels the scheduler and waits up to five seconds for the current run
// to finish.
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		m.mu.Lock()
		running := m.running || m.runner.Busy()
		m.mu.Unlock()

		if !running || time.Now().After(deadline) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// hasManagedCommodity reports whether any commodity has a price provider.
func hasManagedCommodity(cfg config.Config) bool {
	for _, c := range cfg.Commodities {
		if c.Price.Provider != "" {
			return true
		}
	}
	return false
}

// timePtr returns a pointer to t, or nil when t is the zero time.
func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
