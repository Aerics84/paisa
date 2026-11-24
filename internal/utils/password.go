package utils

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Password hash parameters. They are part of the encoded format and are
// validated on verification so that a corrupted or tampered encoding can not
// make verification allocate unbounded memory or CPU.
const (
	argon2IDPrefix = "argon2id"
	argon2Memory   = 64 * 1024 // KiB
	argon2Time     = 3
	argon2Threads  = 2
	argon2SaltLen  = 16
	argon2KeyLen   = 32

	// Upper bounds accepted while verifying an existing encoding.
	argon2MaxMemory    = 1024 * 1024 // KiB, 1 GiB
	argon2MaxTime      = 32
	argon2MaxThreads   = 32
	legacySHA256Prefix = "sha256:"
)

// HashPassword returns an Argon2id hash of password in the format
//
//	argon2id$v=19$m=65536,t=3,p=2$<base64-salt>$<base64-hash>
//
// A fresh 16 byte salt is generated for every call.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	return fmt.Sprintf("argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory,
		argon2Time,
		argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword checks password against encoded. It reports whether the
// password matches (ok) and whether the stored encoding should be upgraded
// (needsRehash). Callers must only rehash when ok is true.
//
// Two encodings are understood: Argon2id as produced by HashPassword, and the
// legacy "sha256:" + Sha256(Sha256(password)) format. Unknown or malformed
// encodings return an error so that they can never verify successfully.
func VerifyPassword(encoded, password string) (ok bool, needsRehash bool, err error) {
	if strings.HasPrefix(encoded, legacySHA256Prefix) {
		expected := legacySHA256Prefix + Sha256(Sha256(password))
		return subtle.ConstantTimeCompare([]byte(encoded), []byte(expected)) == 1, true, nil
	}

	memory, iterations, threads, salt, key, err := parseArgon2ID(encoded)
	if err != nil {
		return false, false, err
	}

	computed := argon2.IDKey([]byte(password), salt, iterations, memory, threads, argon2KeyLen)
	if subtle.ConstantTimeCompare(computed, key) != 1 {
		return false, false, nil
	}

	return true, false, nil
}

func parseArgon2ID(encoded string) (memory, iterations uint32, threads uint8, salt, key []byte, err error) {
	if !strings.HasPrefix(encoded, argon2IDPrefix+"$") {
		return 0, 0, 0, nil, nil, fmt.Errorf("unsupported password hash")
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 5 {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: expected 5 fields, got %d", len(parts))
	}

	if parts[1] != fmt.Sprintf("v=%d", argon2.Version) {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: unsupported version")
	}

	var p uint32
	params := strings.Split(parts[2], ",")
	if len(params) != 3 {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: malformed parameters")
	}

	for _, param := range params {
		name, value, found := strings.Cut(param, "=")
		if !found {
			return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: malformed parameters")
		}

		parsed, parseErr := strconv.ParseUint(value, 10, 32)
		if parseErr != nil {
			return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: malformed parameters")
		}

		switch name {
		case "m":
			memory = uint32(parsed)
		case "t":
			iterations = uint32(parsed)
		case "p":
			p = uint32(parsed)
		default:
			return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: unknown parameter %q", name)
		}
	}

	if memory < 8*p || memory > argon2MaxMemory {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: unsupported memory parameter")
	}
	if iterations < 1 || iterations > argon2MaxTime {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: unsupported time parameter")
	}
	if p < 1 || p > argon2MaxThreads {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: unsupported parallelism parameter")
	}
	threads = uint8(p)

	salt, err = base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: malformed salt")
	}
	if len(salt) != argon2SaltLen {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: unexpected salt length")
	}

	key, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: malformed hash")
	}
	if len(key) != argon2KeyLen {
		return 0, 0, 0, nil, nil, fmt.Errorf("invalid password hash: unexpected hash length")
	}

	return memory, iterations, threads, salt, key, nil
}
