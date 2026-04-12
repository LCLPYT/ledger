package util_test

import (
	"testing"

	"ledger/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePassword_Valid(t *testing.T) {
	require.NoError(t, util.ValidatePassword("Str0ng!Pass"))
}

func TestValidatePassword_Rules(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"too short", "Ab1!"},
		{"no uppercase", "str0ng!pass"},
		{"no lowercase", "STR0NG!PASS"},
		{"no digit", "Strong!Pass"},
		{"no special", "Str0ngPass"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Error(t, util.ValidatePassword(tc.password))
		})
	}
}
