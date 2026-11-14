// Package phone trata telefones brasileiros (DDD + número).
package phone

import (
	"regexp"
	"strings"
)

var allowed = regexp.MustCompile(`^[\d\s()+\-]+$`)

// Normalize devolve só os dígitos nacionais (DDD + número), sem o 55.
// Aceita "(83) 99999-1234", "+55 83 99999-1234" etc.; qualquer outra coisa é inválida.
func Normalize(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if !allowed.MatchString(raw) {
		return "", false
	}
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if (len(d) == 12 || len(d) == 13) && strings.HasPrefix(d, "55") {
		d = d[2:]
	}
	if len(d) != 10 && len(d) != 11 {
		return "", false
	}
	if d[0] == '0' || d[1] == '0' { // DDD vai de 11 a 99
		return "", false
	}
	if len(d) == 11 && d[2] != '9' { // celular começa com 9
		return "", false
	}
	return d, true
}

// WhatsApp devolve o número no formato do wa.me (com o 55 na frente).
func WhatsApp(national string) string { return "55" + national }

// Format escreve um número nacional de forma legível: (83) 99999-1234. O que não parecer telefone volta como veio.
func Format(national string) string {
	switch len(national) {
	case 11:
		return "(" + national[:2] + ") " + national[2:7] + "-" + national[7:]
	case 10:
		return "(" + national[:2] + ") " + national[2:6] + "-" + national[6:]
	default:
		return national
	}
}
