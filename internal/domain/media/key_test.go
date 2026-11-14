package media_test

import (
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/media"
	"github.com/stretchr/testify/assert"
)

func TestNewKey_IsRandomAndValid(t *testing.T) {
	seen := map[string]bool{}
	for _, ext := range []string{".jpg", ".png", ".webp"} {
		for i := 0; i < 50; i++ {
			k := media.NewKey(ext)
			assert.True(t, media.ValidKey(k), k)
			assert.True(t, len(k) > len(ext))
			seen[k] = true
		}
	}
	assert.Len(t, seen, 150, "nunca repete")
}

func TestValidKey_OnlyAcceptsWhatWeGenerate(t *testing.T) {
	good := "3f2b8c1e-5d4a-4b6f-9a1c-0e7d2f8b6a31.webp"
	assert.True(t, media.ValidKey(good))

	for _, bad := range []string{
		"", "../" + good, "a/" + good, "/" + good, good + "/", `..\` + good, "3f2b8c1e.webp", "nao-e-uuid.jpg",
		"3f2b8c1e-5d4a-4b6f-9a1c-0e7d2f8b6a31.svg", "3f2b8c1e-5d4a-4b6f-9a1c-0e7d2f8b6a31.JPG",
		"3f2b8c1e-5d4a-4b6f-9a1c-0e7d2f8b6a31.jpg.exe", good + "\x00", ".htaccess", "index.html",
		"3F2B8C1E-5D4A-4B6F-9A1C-0E7D2F8B6A31.png", " " + good,
	} {
		assert.False(t, media.ValidKey(bad), "%q", bad)
	}
}
