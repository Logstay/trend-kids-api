// Package media valida imagens enviadas pelo painel.
package media

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // registra o decodificador
	_ "image/png"  // registra o decodificador

	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	_ "golang.org/x/image/webp" // registra o decodificador
)

const (
	MaxBytes     = 5 << 20 // 5 MB
	MaxDimension = 8000    // pixels por lado: acima disso o navegador pode travar ao decodificar
)

// Image descreve uma imagem aceita.
type Image struct {
	Ext         string // com ponto: .jpg, .png, .webp
	ContentType string
	Width       int
	Height      int
}

var (
	jpegMagic = []byte{0xFF, 0xD8, 0xFF}
	pngMagic  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}
)

// Detect identifica o tipo pelo CONTEÚDO (os primeiros bytes), nunca pelo nome ou pelo Content-Type
// que o navegador informou, e confere se o arquivo realmente decodifica. Só JPEG, PNG e WebP passam:
// SVG fica de fora de propósito (pode carregar script).
func Detect(data []byte) (Image, error) {
	if len(data) == 0 {
		return Image{}, httperr.RequiredError("o arquivo está vazio")
	}
	if len(data) > MaxBytes {
		return Image{}, httperr.RequiredError("a imagem é grande demais: o máximo é 5 MB")
	}

	var kind Image
	switch {
	case bytes.HasPrefix(data, jpegMagic):
		kind = Image{Ext: ".jpg", ContentType: "image/jpeg"}
	case bytes.HasPrefix(data, pngMagic):
		kind = Image{Ext: ".png", ContentType: "image/png"}
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		kind = Image{Ext: ".webp", ContentType: "image/webp"}
	default:
		return Image{}, httperr.RequiredError("formato não suportado: envie uma foto em JPEG, PNG ou WebP")
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Image{}, httperr.RequiredError("o arquivo de imagem está corrompido ou incompleto")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > MaxDimension || cfg.Height > MaxDimension {
		return Image{}, httperr.RequiredError(fmt.Sprintf("a imagem tem dimensões fora do permitido (máximo %d px por lado)", MaxDimension))
	}
	kind.Width, kind.Height = cfg.Width, cfg.Height
	return kind, nil
}
