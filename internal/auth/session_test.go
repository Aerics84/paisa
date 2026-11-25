package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionLifecycle(t *testing.T) {
	store := NewStore(time.Hour)
	token, expires := store.Create("alice")
	require.NotEmpty(t, token)
	require.True(t, expires.After(time.Now()))

	session, ok := store.Lookup(token)
	require.True(t, ok)
	require.Equal(t, "alice", session.Username)
}

func TestExpiredSessionIsRejected(t *testing.T) {
	store := NewStore(time.Minute)
	token, _ := store.Create("alice")
	store.DeleteExpired(time.Now().Add(2 * time.Minute))

	_, ok := store.Lookup(token)
	require.False(t, ok)
}

func TestDeleteRemovesSession(t *testing.T) {
	store := NewStore(time.Hour)
	token, _ := store.Create("alice")
	store.Delete(token)

	_, ok := store.Lookup(token)
	require.False(t, ok)
}

func TestDeleteByUsernameRemovesOnlyThatUsersSessions(t *testing.T) {
	store := NewStore(time.Hour)
	alice, _ := store.Create("alice")
	carol, _ := store.Create("alice")
	bob, _ := store.Create("bob")

	store.DeleteByUsername("alice")

	_, aliceOK := store.Lookup(alice)
	_, carolOK := store.Lookup(carol)
	_, bobOK := store.Lookup(bob)
	require.False(t, aliceOK)
	require.False(t, carolOK)
	require.True(t, bobOK)
}

func TestClearRemovesEverySession(t *testing.T) {
	store := NewStore(time.Hour)
	alice, _ := store.Create("alice")
	bob, _ := store.Create("bob")

	store.Clear()

	_, aliceOK := store.Lookup(alice)
	_, bobOK := store.Lookup(bob)
	require.False(t, aliceOK)
	require.False(t, bobOK)
}

func TestUnknownTokenIsRejected(t *testing.T) {
	store := NewStore(time.Hour)
	_, ok := store.Lookup("not-a-token")
	require.False(t, ok)
}

func TestConcurrentCreateAndLookup(t *testing.T) {
	store := NewStore(time.Hour)
	results := make(chan bool, 100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, _ := store.Create("alice")
			_, ok := store.Lookup(token)
			results <- ok
		}()
	}
	wg.Wait()
	close(results)
	for ok := range results {
		require.True(t, ok)
	}
}
