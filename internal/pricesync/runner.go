package pricesync

// Runner is a single-flight guard that allows at most one price run at a
// time. It is a lightweight alternative to a mutex: TryStart acquires the
// slot without blocking, Finish releases it, and Run blocks until the slot is
// free.
type Runner struct {
	sem chan struct{}
}

// NewRunner returns an idle Runner.
func NewRunner() *Runner { return &Runner{sem: make(chan struct{}, 1)} }

// TryStart acquires the run slot if it is free. It returns false when another
// run is already in progress.
func (r *Runner) TryStart() bool {
	select {
	case r.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

// Finish releases the run slot acquired by TryStart.
func (r *Runner) Finish() { <-r.sem }

// Busy reports whether a run slot is currently held.
func (r *Runner) Busy() bool { return len(r.sem) > 0 }

// Run executes fn while holding the run slot, blocking until the slot is free.
func (r *Runner) Run(fn func() error) error {
	r.sem <- struct{}{}
	defer r.Finish()
	return fn()
}

// DefaultRunner is the shared runner used by callers that do not manage their
// own instance.
var DefaultRunner = NewRunner()
