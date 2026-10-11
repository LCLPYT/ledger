package admin

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lclpyt/ledger/internal/auth"
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 72
)

// rolePattern mirrors the check constraint on roles.name.
var rolePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

func ValidateUserName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name must not be empty")
	}

	return nil
}

func ValidatePassword(password []byte) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d bytes long", MinPasswordLength)
	}

	if len(password) > MaxPasswordLength {
		return fmt.Errorf("password must be at most %d bytes long", MaxPasswordLength)
	}

	return nil
}

func ValidateRoleName(name string) error {
	if !rolePattern.MatchString(name) {
		return fmt.Errorf("role name has to match %s", rolePattern)
	}

	return nil
}

func ValidatePermissions(permissions []string) error {
	for _, permission := range permissions {
		if !auth.ValidPermission(permission) {
			return fmt.Errorf("invalid permission %q", permission)
		}
	}

	return nil
}
