package solvela

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// SessionInfo holds the current state of a session.
type SessionInfo struct {
	Model     string
	Escalated bool
}

type sessionEntry struct {
	model        string
	created      time.Time
	requestCount int
	recentHashes []uint64
	escalated    bool
}

// SessionStore manages conversation sessions with model escalation.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*sessionEntry
	ttl      time.Duration
}

// NewSessionStore creates a session store with the given TTL.
func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*sessionEntry),
		ttl:      ttl,
	}
}

// GetOrCreate returns session info for the given ID, creating a new session if needed.
// Expired sessions are replaced with fresh ones.
func (s *SessionStore) GetOrCreate(sessionID, defaultModel string) SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.sessions[sessionID]
	if !ok || time.Since(entry.created) > s.ttl {
		s.sessions[sessionID] = &sessionEntry{
			model:        defaultModel,
			created:      time.Now(),
			requestCount: 0,
			recentHashes: make([]uint64, 0),
			escalated:    false,
		}
		return SessionInfo{Model: defaultModel, Escalated: false}
	}

	entry.requestCount++
	return SessionInfo{Model: entry.model, Escalated: entry.escalated}
}

// RecordRequest records a request hash. If 3 or more identical hashes appear,
// the session is escalated (three-strike rule).
func (s *SessionStore) RecordRequest(sessionID string, requestHash uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.sessions[sessionID]
	if !ok {
		return
	}

	entry.recentHashes = append(entry.recentHashes, requestHash)

	// Count occurrences of this hash
	count := 0
	for _, h := range entry.recentHashes {
		if h == requestHash {
			count++
		}
	}
	if count >= 3 {
		entry.escalated = true
	}
}

// CleanupExpired removes all sessions that have exceeded their TTL.
func (s *SessionStore) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, entry := range s.sessions {
		if time.Since(entry.created) > s.ttl {
			delete(s.sessions, id)
		}
	}
}

// DeriveSessionID generates a deterministic session ID from the first message.
func DeriveSessionID(messages []ChatMessage) string {
	h := sha256.New()
	if len(messages) > 0 {
		h.Write([]byte(messages[0].Role))
		h.Write([]byte(messages[0].Content))
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:16])
}
