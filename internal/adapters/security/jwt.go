package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const minSecretLen = 32

type TokenService struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenService(secret string, ttl time.Duration, now func() time.Time) (*TokenService, error) {
	if len(secret) < minSecretLen {
		return nil, errors.New("JWT_SECRET precisa ter ao menos 32 caracteres")
	}
	return &TokenService{secret: []byte(secret), ttl: ttl, now: now}, nil
}

func (s *TokenService) Issue(adminUID string) (string, time.Time, error) {
	issued := s.now()
	expires := issued.Add(s.ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   adminUID,
		IssuedAt:  jwt.NewNumericDate(issued),
		ExpiresAt: jwt.NewNumericDate(expires),
	})
	signed, err := token.SignedString(s.secret)
	return signed, expires, err
}

// Parse valida assinatura (só HS256) e validade; devolve o uid do admin.
func (s *TokenService) Parse(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithTimeFunc(s.now),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", errors.New("token sem sujeito")
	}
	return claims.Subject, nil
}
