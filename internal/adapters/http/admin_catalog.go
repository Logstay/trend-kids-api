package http

import (
	"context"
	nethttp "net/http"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/gin-gonic/gin"
)

type AdminCatalog struct {
	List   ListUseCase
	Create interface {
		Handle(ctx context.Context, cmd ports.SaveProduct) (catalog.Product, error)
	}
	Update interface {
		Handle(ctx context.Context, cmd ports.UpdateProduct) (catalog.Product, error)
	}
	AdjustStock interface {
		Handle(ctx context.Context, cmd ports.AdjustStock) (int, error)
	}
}

type adminCatalogHandler struct{ AdminCatalog }

type createProductRequest struct {
	Name        string `json:"name" binding:"required"`
	Category    string `json:"category" binding:"required"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Stock       int    `json:"stock"`
}

// updateProductRequest é uma substituição completa: Active é ponteiro e obrigatório, para que
// esquecer o campo nunca desative o produto por acidente. Estoque não faz parte (só por ajuste).
type updateProductRequest struct {
	Name        string `json:"name" binding:"required"`
	Category    string `json:"category" binding:"required"`
	Description string `json:"description"`
	PriceCents  int64  `json:"price_cents"`
	Active      *bool  `json:"active" binding:"required"`
}

type stockRequest struct {
	Delta  int    `json:"delta"`
	Reason string `json:"reason"`
}

func (h adminCatalogHandler) List(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	filter.IncludeInactive = true

	products, err := h.AdminCatalog.List.Handle(c.Request.Context(), filter)
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	items := make([]ProductResponse, 0, len(products))
	for _, p := range products {
		items = append(items, toResponse(p))
	}
	c.JSON(nethttp.StatusOK, ListResponse{Items: items})
}

func (h adminCatalogHandler) Create(c *gin.Context) {
	var req createProductRequest
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.AdminCatalog.Create.Handle(c.Request.Context(), ports.SaveProduct{
		Name: req.Name, Category: req.Category, Description: req.Description, PriceCents: req.PriceCents, Stock: req.Stock,
	})
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusCreated, toResponse(p))
}

func (h adminCatalogHandler) Update(c *gin.Context) {
	var req updateProductRequest
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.AdminCatalog.Update.Handle(c.Request.Context(), ports.UpdateProduct{
		UID: c.Param("uid"), Name: req.Name, Category: req.Category, Description: req.Description,
		PriceCents: req.PriceCents, Active: *req.Active,
	})
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toResponse(p))
}

func (h adminCatalogHandler) AdjustStock(c *gin.Context) {
	var req stockRequest
	if !bindJSON(c, &req) {
		return
	}
	stock, err := h.AdminCatalog.AdjustStock.Handle(c.Request.Context(), ports.AdjustStock{
		UID: c.Param("uid"), Delta: req.Delta, Reason: req.Reason,
		AdminUID: c.GetString(adminIDKey), // sempre da sessão, nunca do corpo
	})
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, gin.H{"stock": stock})
}
