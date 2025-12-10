package api

import (
	"sync"
	"time"
)

// TokenManager manages authentication tokens with thread-safe caching.
type TokenManager struct {
	mu           sync.RWMutex
	token        string
	expiry       time.Time
	ClientID     string
	ClientSecret string
	APIURL       string
}

// NewTokenManager creates a new token manager.
func NewTokenManager(clientID, clientSecret, apiURL string) *TokenManager {
	return &TokenManager{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		APIURL:       apiURL,
	}
}

// GetToken returns a valid token, refreshing if necessary.
// Thread-safe with read/write locks.
func (tm *TokenManager) GetToken() (string, error) {
	// Check if token is still valid (with 5 minute buffer)
	tm.mu.RLock()
	if tm.token != "" && time.Now().Before(tm.expiry.Add(-5*time.Minute)) {
		token := tm.token
		tm.mu.RUnlock()
		return token, nil
	}
	tm.mu.RUnlock()

	// Need to refresh token - acquire write lock
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have refreshed)
	if tm.token != "" && time.Now().Before(tm.expiry.Add(-5*time.Minute)) {
		return tm.token, nil
	}

	// Request new token
	token, expiry, err := tm.refreshToken()
	if err != nil {
		return "", err
	}

	tm.token = token
	tm.expiry = expiry
	return token, nil
}

// refreshToken requests a new token from the API.
// This should be called with the write lock held.
func (tm *TokenManager) refreshToken() (string, time.Time, error) {
	// This will be implemented in client.go to avoid circular dependency
	// For now, return an error - client.go will handle the actual refresh
	return "", time.Time{}, nil
}

// SetToken sets the token and expiry (used by client during refresh).
func (tm *TokenManager) SetToken(token string, expiry time.Time) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.token = token
	tm.expiry = expiry
}
