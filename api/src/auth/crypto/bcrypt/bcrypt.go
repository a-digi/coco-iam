package bcrypt

import (
	"errors"
	"golang.org/x/crypto/bcrypt"
)

// DefaultCost is the recommended bcrypt cost. Tune to ~100ms on your prod hardware.
const DefaultCost = 12

// HashPassword hashes a plaintext password using bcrypt with the provided cost.
// Returns the bcrypt hash as a string.
func HashPassword(plaintext string, cost int) (string, error) {
	if plaintext == "" {
		return "", errors.New("password must not be empty")
	}
	if cost == 0 {
		cost = DefaultCost
	}
	// GenerateFromPassword returns []byte hash
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword compares a bcrypt hash with a plaintext password.
// Returns nil on success; error when mismatch or invalid hash.
func VerifyPassword(hash string, plaintext string) error {
	if hash == "" {
		return errors.New("hash must not be empty")
	}
	if plaintext == "" {
		return errors.New("password must not be empty")
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}

// NeedsRehash checks whether the given bcrypt hash was created with a lower cost
// than desired. If so, you should re-hash on next successful login.
func NeedsRehash(hash string, desiredCost int) (bool, error) {
	if desiredCost == 0 {
		desiredCost = DefaultCost
	}

	currentCost, err := bcrypt.Cost([]byte(hash))

	if err != nil {
		return false, err
	}

	return currentCost < desiredCost, nil
}
