package database

import (
	crypto_bcrypt "github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	password_entity "github.com/a-digi/coco-iam/src/auth/database/entity"
)

type PasswordQueryRepository interface {
	FindByUserID(userID string) (*password_entity.Password, bool, error)
}

type PasswordVerifier func(hash string, plaintext string) error

type PasswordAuthenticator struct {
	Verifier     PasswordVerifier
	PasswordRepo PasswordQueryRepository
}

func NewPasswordAuthenticator(repo PasswordQueryRepository) *PasswordAuthenticator {
	return &PasswordAuthenticator{
		Verifier:     crypto_bcrypt.VerifyPassword,
		PasswordRepo: repo,
	}
}

func (a *PasswordAuthenticator) Verify(userID string, plaintext string) (bool, error) {

	if a == nil || a.PasswordRepo == nil || a.Verifier == nil {
		return false, nil
	}

	pw, found, err := a.PasswordRepo.FindByUserID(userID)

	if err != nil {
		return false, err
	}

	if !found || pw == nil || !pw.IsActive {
		return false, nil
	}

	if err := a.Verifier(pw.Password, plaintext); err != nil {
		return false, nil
	}

	return true, nil
}
