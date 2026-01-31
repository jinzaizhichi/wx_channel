package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
	"wx_channel/hub_server/database"
)

// BindingManager manages short-lived tokens for device binding
type BindingManager struct {
	tokens map[string]uint // token -> userID
	mu     sync.RWMutex
}

var Binder = &BindingManager{
	tokens: make(map[string]uint),
}

// GenerateToken creates a short code (e.g. 6 chars) valid for 5 minutes
func (bm *BindingManager) GenerateToken(userID uint) (string, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Generate 3 random bytes = 6 hex chars
	bytes := make([]byte, 3)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	bm.tokens[token] = userID

	// Auto expire after 5 mins
	go func(t string) {
		time.Sleep(5 * time.Minute)
		bm.mu.Lock()
		delete(bm.tokens, t)
		bm.mu.Unlock()
	}(token)

	return token, nil
}

// ValidateToken returns the UserID if valid, and consumes the token
func (bm *BindingManager) ValidateAndConsume(token string) (uint, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	userID, ok := bm.tokens[token]
	if !ok {
		return 0, errors.New("invalid or expired token")
	}

	delete(bm.tokens, token) // One-time use
	return userID, nil
}

// ProcessBindRequest validates the token and binds the node to the user
func ProcessBindRequest(nodeID string, token string) error {
	userID, err := Binder.ValidateAndConsume(token)
	if err != nil {
		return err
	}

	// Update Node in DB
	return database.UpdateNodeBinding(nodeID, userID)
}

// Variable to be set by main or init to avoid circular import if database imports services?
// checking deps: database imports models. services imports database?
// Let's check db.go imports.
