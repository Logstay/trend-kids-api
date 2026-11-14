package media_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/media"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil))
	return buf.Bytes()
}

// WebP lossless de 1x1 pixel.
func webpBytes(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	require.NoError(t, err)
	return b
}

func TestDetect_AcceptsJPEGPNGAndWebPByTheirRealContent(t *testing.T) {
	tests := map[string]struct {
		data        []byte
		ext         string
		contentType string
	}{
		"jpeg": {jpegBytes(t), ".jpg", "image/jpeg"},
		"png":  {pngBytes(t, 8, 8), ".png", "image/png"},
		"webp": {webpBytes(t), ".webp", "image/webp"},
	}
	for name, tc := range tests {
		got, err := media.Detect(tc.data)
		require.NoError(t, err, name)
		assert.Equal(t, tc.ext, got.Ext, name)
		assert.Equal(t, tc.contentType, got.ContentType, name)
	}
}

func TestDetect_RejectsEverythingElseEvenIfItClaimsToBeAnImage(t *testing.T) {
	tests := map[string][]byte{
		"svg (pode carregar script)": []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html disfarçado":            []byte(`<html><script>alert(1)</script></html>`),
		"gif":                        []byte("GIF89a\x01\x00\x01\x00"),
		"texto":                      []byte("isto não é uma imagem"),
		"vazio":                      {},
		"só o cabeçalho do jpeg":     {0xFF, 0xD8, 0xFF},
		"png truncado":               pngBytes(t, 8, 8)[:20],
		"pdf":                        []byte("%PDF-1.7 ..."),
		"executável":                 []byte("MZ\x90\x00\x03\x00"),
	}
	for name, data := range tests {
		_, err := media.Detect(data)
		var req httperr.RequiredError
		assert.True(t, errors.As(err, &req), name)
	}
}

func TestDetect_RejectsFilesOverTheSizeLimit(t *testing.T) {
	big := append(pngBytes(t, 8, 8), bytes.Repeat([]byte{0}, media.MaxBytes)...)

	_, err := media.Detect(big)

	var req httperr.RequiredError
	require.True(t, errors.As(err, &req))
	assert.Contains(t, err.Error(), "5 MB")
}

// Uma imagem minúscula em bytes pode declarar dimensões gigantes (bomba de descompressão no navegador).
func TestDetect_RejectsAbsurdDimensions(t *testing.T) {
	data := pngBytes(t, 8, 8)
	// IHDR: largura e altura (4 bytes cada) começam no byte 16
	binary.BigEndian.PutUint32(data[16:20], 30000)
	binary.BigEndian.PutUint32(data[20:24], 30000)
	// o decodificador confere o CRC do bloco IHDR (tipo + dados = bytes 12..29); recalcula para o cabeçalho continuar válido
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))

	_, err := media.Detect(data)

	var req httperr.RequiredError
	require.True(t, errors.As(err, &req))
	assert.Contains(t, err.Error(), "dimensões")
}

func TestDetect_ErrorMessagesAreFriendly(t *testing.T) {
	_, err := media.Detect([]byte("texto"))
	assert.Contains(t, strings.ToLower(err.Error()), "jpeg")
}
