package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/gofrs/uuid/v5"
)

const (
	tokenPrefix    = "ldg_"
	secretBytes    = 32
	idHexLength    = 32
	secretB64Len   = 43
	tokenLength    = len(tokenPrefix) + idHexLength + 1 + secretB64Len
	tokenSeparator = '_'
)

// NewToken returns a token of the form ldg_<key id>_<secret> together with the hash of its secret
func NewToken() (token string, id uuid.UUID, secretHash []byte, err error) {
	id, err = uuid.NewV4()

	if err != nil {
		return "", uuid.Nil, nil, err
	}

	raw := make([]byte, secretBytes)

	if _, err := rand.Read(raw); err != nil {
		return "", uuid.Nil, nil, err
	}

	secret := base64.RawURLEncoding.EncodeToString(raw)
	token = tokenPrefix + hex.EncodeToString(id.Bytes()) + string(tokenSeparator) + secret

	return token, id, HashSecret(secret), nil
}

func ParseToken(token string) (id uuid.UUID, secret string, ok bool) {
	rest, found := strings.CutPrefix(token, tokenPrefix)

	if !found || len(token) != tokenLength || rest[idHexLength] != tokenSeparator {
		return uuid.Nil, "", false
	}

	idBytes, err := hex.DecodeString(rest[:idHexLength])

	if err != nil {
		return uuid.Nil, "", false
	}

	id, err = uuid.FromBytes(idBytes)

	if err != nil {
		return uuid.Nil, "", false
	}

	return id, rest[idHexLength+1:], true
}

// HashSecret uses a plain SHA-256, since a random 256 bit secret needs no key stretching.
func HashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))

	return sum[:]
}
