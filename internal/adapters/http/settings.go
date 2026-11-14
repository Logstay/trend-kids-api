package http

import (
	"context"
	nethttp "net/http"

	"github.com/Logstay/trend-kids-api/internal/domain/settings"
	ports "github.com/Logstay/trend-kids-api/internal/ports/settings"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/Logstay/trend-kids-api/pkg/phone"
	"github.com/gin-gonic/gin"
)

// SettingsRoutes liga GET /v1/settings (público) e PUT /v1/admin/settings (sessão).
type SettingsRoutes struct {
	Get interface {
		Handle(ctx context.Context) (settings.Settings, error)
	}
	Update interface {
		Handle(ctx context.Context, cmd ports.UpdateSettings) (settings.Settings, error)
	}
}

// SettingsResponse já vem pronta para o site: links e textos formatados.
type SettingsResponse struct {
	Name        string `json:"name"`
	City        string `json:"city"`
	Description string `json:"description"`
	// WhatsApp no formato do wa.me (55 + DDD + número); vazio = não configurado.
	WhatsApp        string `json:"whatsapp"`
	WhatsAppDisplay string `json:"whatsapp_display"`
	Phone           string `json:"phone"`
	PhoneTel        string `json:"phone_tel"` // para links tel:, ex.: +558332221234
	Instagram       string `json:"instagram"`
	InstagramURL    string `json:"instagram_url"`
}

func toSettingsResponse(s settings.Settings) SettingsResponse {
	res := SettingsResponse{
		Name: s.Name(), City: s.City(), Description: s.Description(),
		Instagram: s.Instagram(), InstagramURL: settings.InstagramURL(s.Instagram()),
	}
	if s.WhatsApp() != "" {
		res.WhatsApp = phone.WhatsApp(s.WhatsApp())
		res.WhatsAppDisplay = phone.Format(s.WhatsApp())
	}
	if s.Phone() != "" {
		res.Phone = phone.Format(s.Phone())
		res.PhoneTel = "+" + phone.WhatsApp(s.Phone())
	}
	return res
}

// updateSettingsRequest é substituição completa: os contatos são ponteiros obrigatórios, então
// esquecer a chave dá 400 (e "" apaga de propósito). Assim um cliente antigo nunca apaga um contato sem querer.
type updateSettingsRequest struct {
	Name        string  `json:"name" binding:"required"`
	City        string  `json:"city" binding:"required"`
	Description *string `json:"description" binding:"required"`
	WhatsApp    *string `json:"whatsapp" binding:"required"`
	Phone       *string `json:"phone" binding:"required"`
	Instagram   *string `json:"instagram" binding:"required"`
}

type settingsHandler struct{ SettingsRoutes }

func (h settingsHandler) Get(c *gin.Context) {
	s, err := h.SettingsRoutes.Get.Handle(c.Request.Context())
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.Header("Cache-Control", "public, max-age=30")
	c.JSON(nethttp.StatusOK, toSettingsResponse(s))
}

func (h settingsHandler) Put(c *gin.Context) {
	var req updateSettingsRequest
	if !bindJSON(c, &req) {
		return
	}
	s, err := h.Update.Handle(c.Request.Context(), ports.UpdateSettings{
		Name: req.Name, City: req.City, Description: *req.Description,
		WhatsApp: *req.WhatsApp, Phone: *req.Phone, Instagram: *req.Instagram,
		AdminUID: c.GetString(adminIDKey), // sempre da sessão, nunca do corpo
	})
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toSettingsResponse(s))
}
