package service

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

var bcryptCost = bcrypt.DefaultCost

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}
