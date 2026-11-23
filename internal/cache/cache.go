package cache

import (
	"github.com/ananthakumaran/paisa/internal/accounting"
	"github.com/ananthakumaran/paisa/internal/model/transaction"
	"github.com/ananthakumaran/paisa/internal/prediction"
	"github.com/ananthakumaran/paisa/internal/service"
)

// clearHook is an optional callback invoked after the in-memory caches are
// cleared. Tests install it with SetClearHookForTest to observe invalidations
// without reaching into the individual cache implementations.
var clearHook func()

// SetClearHookForTest installs a callback that Clear invokes after clearing the
// caches. Pass nil to remove it. It is intended for tests only.
func SetClearHookForTest(fn func()) { clearHook = fn }

func Clear() {
	service.ClearInterestCache()
	service.ClearPriceCache()
	accounting.ClearCache()
	prediction.ClearCache()
	transaction.ClearCache()
	if clearHook != nil {
		clearHook()
	}
}
