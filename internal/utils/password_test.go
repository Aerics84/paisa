package utils

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(encoded, "argon2id$v=19$m=65536,t=3,p=2$"))

	ok, needsRehash, err := VerifyPassword(encoded, "correct horse battery staple")
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, needsRehash)
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	encoded, err := HashPassword("secret")
	require.NoError(t, err)

	ok, _, err := VerifyPassword(encoded, "wrong")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerifyPasswordAcceptsLegacySHA256AndRequestsRehash(t *testing.T) {
	legacy := "sha256:" + Sha256(Sha256("secret"))

	ok, needsRehash, err := VerifyPassword(legacy, "secret")
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, needsRehash)
}

func TestVerifyPasswordRejectsMalformedEncodings(t *testing.T) {
	validSalt := base64.RawStdEncoding.EncodeToString(make([]byte, argon2SaltLen))
	validKey := base64.RawStdEncoding.EncodeToString(make([]byte, argon2KeyLen))
	shortSalt := base64.RawStdEncoding.EncodeToString(make([]byte, argon2SaltLen/2))
	shortKey := base64.RawStdEncoding.EncodeToString(make([]byte, argon2KeyLen/2))

	encode := func(version, params, salt, key string) string {
		return fmt.Sprintf("argon2id$%s$%s$%s$%s", version, params, salt, key)
	}

	tests := []struct {
		name    string
		encoded string
	}{
		{"empty", ""},
		{"unknown prefix", "secret"},
		{"bcrypt", "bcrypt$v=19$m=65536,t=3,p=2$" + validSalt + "$" + validKey},
		{"sha256 without colon", "sha256"},
		{"unsupported version", encode("v=18", "m=65536,t=3,p=2", validSalt, validKey)},
		{"parameter missing equals", encode("v=19", "m=65536,t=3,p", validSalt, validKey)},
		{"wrong parameter count", encode("v=19", "m=65536,t=3", validSalt, validKey)},
		{"non-numeric memory", encode("v=19", "m=abc,t=3,p=2", validSalt, validKey)},
		{"non-numeric time", encode("v=19", "m=65536,t=abc,p=2", validSalt, validKey)},
		{"non-numeric parallelism", encode("v=19", "m=65536,t=3,p=abc", validSalt, validKey)},
		{"unknown parameter", encode("v=19", "m=65536,t=3,q=2", validSalt, validKey)},
		{"zero memory", encode("v=19", "m=0,t=3,p=2", validSalt, validKey)},
		{"memory below 8*parallelism", encode("v=19", "m=63,t=3,p=8", validSalt, validKey)},
		{"memory above max", encode("v=19", "m=1048576,t=3,p=2", validSalt, validKey)},
		{"zero time", encode("v=19", "m=65536,t=0,p=2", validSalt, validKey)},
		{"time above max", encode("v=19", "m=65536,t=32,p=2", validSalt, validKey)},
		{"zero parallelism", encode("v=19", "m=65536,t=3,p=0", validSalt, validKey)},
		{"parallelism above max", encode("v=19", "m=65536,t=3,p=9", validSalt, validKey)},
		{"bad salt base64", encode("v=19", "m=65536,t=3,p=2", "!!!!", validKey)},
		{"wrong salt length", encode("v=19", "m=65536,t=3,p=2", shortSalt, validKey)},
		{"bad key base64", encode("v=19", "m=65536,t=3,p=2", validSalt, "!!!!")},
		{"wrong key length", encode("v=19", "m=65536,t=3,p=2", validSalt, shortKey)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, needsRehash, err := VerifyPassword(tt.encoded, "secret")
			require.Error(t, err)
			require.False(t, ok)
			require.False(t, needsRehash)
		})
	}
}

func TestVerifyPasswordRejectsMalformedLegacyHash(t *testing.T) {
	for _, encoded := range []string{
		"sha256:",
		"sha256:zzz",
		"sha256:abc",
		"sha256:" + strings.Repeat("A", 64),
		"sha256:" + Sha256(Sha256("secret")) + "0",
	} {
		ok, needsRehash, err := VerifyPassword(encoded, "secret")
		require.Error(t, err, "encoding %q must not verify", encoded)
		require.False(t, ok)
		require.False(t, needsRehash)
	}
}

func TestVerifyPasswordWrongLegacyPasswordDoesNotRequestRehash(t *testing.T) {
	legacy := "sha256:" + Sha256(Sha256("secret"))

	ok, needsRehash, err := VerifyPassword(legacy, "wrong")
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, needsRehash)
}

func TestHashPasswordUsesFreshSalt(t *testing.T) {
	first, err := HashPassword("secret")
	require.NoError(t, err)
	second, err := HashPassword("secret")
	require.NoError(t, err)
	require.NotEqual(t, first, second)

	for _, encoded := range []string{first, second} {
		ok, needsRehash, err := VerifyPassword(encoded, "secret")
		require.NoError(t, err)
		require.True(t, ok)
		require.False(t, needsRehash)
	}
}

func TestHashPasswordEncodingShape(t *testing.T) {
	encoded, err := HashPassword("secret")
	require.NoError(t, err)

	parts := strings.Split(encoded, "$")
	require.Len(t, parts, 5)
	require.Equal(t, "argon2id", parts[0])
	require.Equal(t, "v=19", parts[1])
	require.Equal(t, "m=65536,t=3,p=2", parts[2])

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	require.NoError(t, err)
	require.Len(t, salt, argon2SaltLen)

	key, err := base64.RawStdEncoding.DecodeString(parts[4])
	require.NoError(t, err)
	require.Len(t, key, argon2KeyLen)
}
