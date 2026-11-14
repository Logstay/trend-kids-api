package http

import (
	"context"
	nethttp "net/http"
	"time"

	ports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/gin-gonic/gin"
)

const (
	sessionCookie = "session"
	adminIDKey    = "adminID"
	maxBodyBytes  = 1 << 20
)

type LoginUseCase interface {
	Handle(ctx context.Context, cmd ports.LoginCommand) (ports.Session, error)
}

type TokenParser interface {
	Parse(token string) (adminUID string, err error)
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type adminHandler struct {
	login        LoginUseCase
	tokens       TokenParser
	cookieSecure bool
	now          func() time.Time
}

func (h adminHandler) Login(c *gin.Context) {
	var req loginRequest
	if !bindJSON(c, &req) {
		return
	}

	session, err := h.login.Handle(c.Request.Context(), ports.LoginCommand{Email: req.Email, Password: req.Password})
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}

	nethttp.SetCookie(c.Writer, h.cookie(session.Token, int(session.ExpiresAt.Sub(h.now()).Seconds())))
	c.Status(nethttp.StatusNoContent)
}

func (h adminHandler) Logout(c *gin.Context) {
	nethttp.SetCookie(c.Writer, h.cookie("", -1))
	c.Status(nethttp.StatusNoContent)
}

func (h adminHandler) Me(c *gin.Context) {
	c.JSON(nethttp.StatusOK, gin.H{"id": c.GetString(adminIDKey)})
}

// RequireAdmin barra qualquer rota sem sessão válida.
func (h adminHandler) RequireAdmin(c *gin.Context) {
	token, err := c.Cookie(sessionCookie)
	if err != nil {
		httperr.ErrorHandler(httperr.UnauthorizedError("sessão ausente"), c)
		return
	}
	uid, err := h.tokens.Parse(token)
	if err != nil {
		httperr.ErrorHandler(httperr.UnauthorizedError("sessão inválida ou expirada"), c)
		return
	}
	c.Set(adminIDKey, uid)
	c.Next()
}

func (h adminHandler) cookie(value string, maxAge int) *nethttp.Cookie {
	return &nethttp.Cookie{
		Name: sessionCookie, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: h.cookieSecure, SameSite: nethttp.SameSiteLaxMode,
	}
}

func rateLimit(l *rateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(c.ClientIP()) {
			httperr.ErrorHandler(httperr.TooManyRequestsError("muitas tentativas; tente de novo em instantes"), c)
			return
		}
		c.Next()
	}
}

// bindJSON exige Content-Type JSON (defesa extra contra CSRF por formulário), limita o corpo
// e valida os campos obrigatórios. Em caso de falha já responde 400 e devolve false.
func bindJSON(c *gin.Context, dst any) bool {
	if c.ContentType() != "application/json" {
		httperr.ErrorHandler(httperr.BadRequestError("Content-Type deve ser application/json"), c)
		return false
	}
	c.Request.Body = nethttp.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	if err := c.ShouldBindJSON(dst); err != nil {
		httperr.ErrorHandler(httperr.BadRequestError("corpo da requisição inválido"), c)
		return false
	}
	return true
}
