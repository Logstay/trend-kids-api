package http

import (
	"context"
	nethttp "net/http"
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
	ports "github.com/Logstay/trend-kids-api/internal/ports/sellers"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/Logstay/trend-kids-api/pkg/phone"
	"github.com/gin-gonic/gin"
)

// SellersRoutes liga GET /v1/sellers (público) e o CRUD em /v1/admin/sellers (sessão).
type SellersRoutes struct {
	List interface {
		Handle(ctx context.Context, onlyActive bool) ([]sellers.Seller, error)
	}
	Create interface {
		Handle(ctx context.Context, cmd ports.CreateSeller) (sellers.Seller, error)
	}
	Update interface {
		Handle(ctx context.Context, cmd ports.UpdateSeller) (sellers.Seller, error)
	}
	Delete interface {
		Handle(ctx context.Context, uid string) error
	}
}

// PublicSellerResponse é o que o site mostra no seletor: só nome e número (nada interno).
type PublicSellerResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	WhatsApp        string `json:"whatsapp"` // formato do wa.me (55 + DDD + número)
	WhatsAppDisplay string `json:"whatsapp_display"`
}

type PublicSellerList struct {
	Items []PublicSellerResponse `json:"items"`
}

// SellerResponse é a visão do painel.
type SellerResponse struct {
	PublicSellerResponse
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type SellerList struct {
	Items []SellerResponse `json:"items"`
}

func toPublicSeller(s sellers.Seller) PublicSellerResponse {
	return PublicSellerResponse{ID: s.UID(), Name: s.Name(), WhatsApp: phone.WhatsApp(s.WhatsApp()), WhatsAppDisplay: phone.Format(s.WhatsApp())}
}

func toSeller(s sellers.Seller) SellerResponse {
	return SellerResponse{PublicSellerResponse: toPublicSeller(s), Active: s.Active(), CreatedAt: s.CreatedAt()}
}

type createSellerRequest struct {
	Name     string `json:"name" binding:"required"`
	WhatsApp string `json:"whatsapp" binding:"required"`
}

// updateSellerRequest é substituição completa: active é ponteiro obrigatório, então esquecer o campo
// dá 400 em vez de desativar a vendedora sem querer.
type updateSellerRequest struct {
	Name     string `json:"name" binding:"required"`
	WhatsApp string `json:"whatsapp" binding:"required"`
	Active   *bool  `json:"active" binding:"required"`
}

type sellersHandler struct{ SellersRoutes }

func (h sellersHandler) PublicList(c *gin.Context) {
	list, err := h.List.Handle(c.Request.Context(), true)
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	items := make([]PublicSellerResponse, 0, len(list))
	for _, s := range list {
		items = append(items, toPublicSeller(s))
	}
	c.Header("Cache-Control", "public, max-age=30")
	c.JSON(nethttp.StatusOK, PublicSellerList{Items: items})
}

func (h sellersHandler) AdminList(c *gin.Context) {
	list, err := h.List.Handle(c.Request.Context(), false)
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	items := make([]SellerResponse, 0, len(list))
	for _, s := range list {
		items = append(items, toSeller(s))
	}
	c.JSON(nethttp.StatusOK, SellerList{Items: items})
}

func (h sellersHandler) CreateSeller(c *gin.Context) {
	var req createSellerRequest
	if !bindJSON(c, &req) {
		return
	}
	s, err := h.Create.Handle(c.Request.Context(), ports.CreateSeller{Name: req.Name, WhatsApp: req.WhatsApp})
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusCreated, toSeller(s))
}

func (h sellersHandler) UpdateSeller(c *gin.Context) {
	var req updateSellerRequest
	if !bindJSON(c, &req) {
		return
	}
	s, err := h.Update.Handle(c.Request.Context(), ports.UpdateSeller{UID: c.Param("uid"), Name: req.Name, WhatsApp: req.WhatsApp, Active: *req.Active})
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toSeller(s))
}

func (h sellersHandler) DeleteSeller(c *gin.Context) {
	if err := h.Delete.Handle(c.Request.Context(), c.Param("uid")); err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.Status(nethttp.StatusNoContent)
}
