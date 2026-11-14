package phone_test

import (
	"testing"

	"github.com/Logstay/trend-kids-api/pkg/phone"
	"github.com/stretchr/testify/assert"
)

func TestNormalize_ReturnsNationalDigits(t *testing.T) {
	for in, want := range map[string]string{
		"(83) 99999-1234":   "83999991234",
		"+55 83 99999-1234": "83999991234",
		"83 3222-1234":      "8332221234",
		"5583999991234":     "83999991234",
		"  83999991234  ":   "83999991234",
	} {
		got, ok := phone.Normalize(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
}

func TestNormalize_RejectsInvalid(t *testing.T) {
	for _, in := range []string{"", "123", "83 9999x-1234", "00999991234", "83899991234", "558399999123456", "abc"} {
		_, ok := phone.Normalize(in)
		assert.False(t, ok, in)
	}
}

func TestWhatsApp_AddsCountryCodeForWaMe(t *testing.T) {
	assert.Equal(t, "5583999991234", phone.WhatsApp("83999991234"))
}

func TestFormat_HumanReadable(t *testing.T) {
	assert.Equal(t, "(83) 99999-1234", phone.Format("83999991234"))
	assert.Equal(t, "(83) 3222-1234", phone.Format("8332221234"))
	assert.Equal(t, "", phone.Format(""))
	assert.Equal(t, "abc", phone.Format("abc"), "o que não é telefone volta como veio")
}
