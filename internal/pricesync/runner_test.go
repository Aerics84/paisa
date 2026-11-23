package pricesync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunnerAllowsOnlyOnePriceRun(t *testing.T) {
	r := NewRunner()
	assert.True(t, r.TryStart())
	assert.False(t, r.TryStart())
	assert.True(t, r.Busy())
	r.Finish()
	assert.False(t, r.Busy())
	assert.True(t, r.TryStart())
	r.Finish()
}

func TestRunnerRunBlocksSecondRun(t *testing.T) {
	r := NewRunner()
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})

	go func() {
		_ = r.Run(func() error {
			close(started)
			<-release
			return nil
		})
		close(done)
	}()
	<-started

	second := make(chan struct{})
	go func() {
		_ = r.Run(func() error { close(second); return nil })
	}()

	select {
	case <-second:
		t.Fatal("second run started before first finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	<-done
	<-second
}
