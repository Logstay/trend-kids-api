package security_test

import (
	"testing"

	"github.com/Logstay/trend-kids-api/internal/adapters/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgon2_HashAndVerify(t *testing.T) {
	h := security.NewArgon2Hasher()

	hash, err := h.Hash("senha-forte-123")
	require.NoError(t, err)

	assert.NotContains(t, hash, "senha-forte-123")
	assert.True(t, h.Verify("senha-forte-123", hash))
	assert.False(t, h.Verify("senha-errada", hash))
}

func TestArgon2_SamePasswordGivesDifferentHashes(t *testing.T) {
	h := security.NewArgon2Hasher()
	a, _ := h.Hash("mesma-senha-123")
	b, _ := h.Hash("mesma-senha-123")
	assert.NotEqual(t, a, b, "salt aleatório por hash")
}

func TestArgon2_MalformedHashNeverVerifiesNorPanics(t *testing.T) {
	h := security.NewArgon2Hasher()
	for _, bad := range []string{"", "texto-qualquer", "$argon2id$v=19$m=1,t=1,p=1$xx$yy", "$bcrypt$abc"} {
		assert.False(t, h.Verify("qualquer", bad), bad)
	}
}
