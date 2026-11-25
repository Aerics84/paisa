package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// Session is an authenticated user session.
type Session struct {
	Username  string
	ExpiresAt time.Time
}

// Store is an in-memory, concurrency-safe session store.
type Store struct {
	ttl      time.Duration
	mu       sync.Mutex
	sessions map[string]Session
}

// NewStore returns a session store whose sessions expire after ttl.
func NewStore(ttl time.Duration) *Store {
	return &Store{
		ttl:      ttl,
		sessions: make(map[string]Session),
	}
}

// Create stores a new session for username and returns its token and expiry.
func (s *Store) Create(username string) (token string, expires time.Time) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic("auth: cannot read random bytes: " + err.Error())
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	expires = time.Now().Add(s.ttl)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = Session{Username: username, ExpiresAt: expires}

	return token, expires
}

// Lookup returns the session for token. Expired sessions are removed and
// reported as missing.
func (s *Store) Lookup(token string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[token]
	if !ok {
		return Session{}, false
	}
	if !time.Now().Before(session.ExpiresAt) {
		delete(s.sessions, token)
		return Session{}, false
	}

	return session, true
}

// Delete removes the session for token, if any.
func (s *Store) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// DeleteByUsername removes every session belonging to username.
func (s *Store) DeleteByUsername(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for token, session := range s.sessions {
		if session.Username == username {
			delete(s.sessions, token)
		}
	}
}

// Clear removes every session. It is used when the set of accounts or a
// password changes so that no previously issued cookie survives the change.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions = make(map[string]Session)
}

// DeleteExpired removes every session that has expired at now.
func (s *Store) DeleteExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for token, session := range s.sessions {
		if !now.Before(session.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
}
