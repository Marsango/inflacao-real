// Package admin provides the single-owner catalog review API.
package admin

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

type password struct {
	memory, iterations uint32
	parallelism        uint8
	salt, digest       []byte
}

func parsePassword(encoded string) (password, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return password{}, fmt.Errorf("ADMIN_PASSWORD_HASH must use Argon2id v19")
	}
	var result password
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &result.memory, &result.iterations, &result.parallelism); err != nil {
		return result, err
	}
	return decodePassword(result, parts)
}

func decodePassword(result password, parts []string) (password, error) {
	var err error
	result.salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return result, err
	}
	result.digest, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return result, err
	}
	if !result.valid() {
		return result, fmt.Errorf("invalid Argon2id parameters: %s", strconv.Itoa(len(result.digest)))
	}
	return result, nil
}

func (hash password) valid() bool {
	return hash.memory >= 19456 && hash.memory <= 262144 && hash.iterations >= 2 && hash.iterations <= 10 &&
		hash.parallelism >= 1 && hash.parallelism <= 8 && len(hash.salt) >= 16 && len(hash.salt) <= 64 && len(hash.digest) == 32
}

func (hash password) verify(value string) bool {
	digest := argon2.IDKey([]byte(value), hash.salt, hash.iterations, hash.memory, hash.parallelism, uint32(len(hash.digest)))
	return subtle.ConstantTimeCompare(digest, hash.digest) == 1
}
