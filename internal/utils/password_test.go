package utils

import (
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

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	_, _, err := VerifyPassword("argon2id$broken", "secret")
	require.Error(t, err)
}

func TestVerifyPasswordRejectsUnknownEncoding(t *testing.T) {
	for _, encoded := range []string{"", "secret", "bcrypt$v=19$m=65536,t=3,p=2", "sha256"} {
		ok, needsRehash, err := VerifyPassword(encoded, "secret")
		require.Error(t, err, "encoding %q must not verify", encoded)
		require.False(t, ok)
		require.False(t, needsRehash)
	}
}

func TestVerifyPasswordLegacyRejectsWrongPassword(t *testing.T) {
	legacy := "sha256:" + Sha256(Sha256("secret"))

	ok, _, err := VerifyPassword(legacy, "wrong")
	require.NoError(t, err)
	require.False(t, ok)
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
