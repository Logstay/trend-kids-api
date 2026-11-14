package media

import (
	"regexp"

	"github.com/google/uuid"
)

// A chave do arquivo é SEMPRE gerada aqui (nunca vem do cliente): uuid minúsculo + extensão conhecida.
// Isso torna impossível um nome com "../", barras ou extensão perigosa chegar ao disco.
var keyRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp)$`)

// NewKey gera uma chave nova para a extensão (.jpg, .png ou .webp).
func NewKey(ext string) string { return uuid.NewString() + ext }

// ValidKey diz se a chave tem exatamente o formato que NewKey gera.
func ValidKey(key string) bool { return keyRe.MatchString(key) }
