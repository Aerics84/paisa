package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClearInvokesHook(t *testing.T) {
	count := 0
	SetClearHookForTest(func() { count++ })
	t.Cleanup(func() { SetClearHookForTest(nil) })

	Clear()
	require.Equal(t, 1, count)

	Clear()
	require.Equal(t, 2, count)
}

func TestClearWithoutHookDoesNotPanic(t *testing.T) {
	SetClearHookForTest(nil)
	t.Cleanup(func() { SetClearHookForTest(nil) })

	require.NotPanics(t, func() { Clear() })
}
