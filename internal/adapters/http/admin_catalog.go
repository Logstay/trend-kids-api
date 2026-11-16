package http

import (
	"context"
	"io"
	nethttp "net/http"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	"github.com/Logstay/trend-kids-api/internal/domain/media"
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
	// Os três abaixo são opcionais: as rotas de galeria só existem se forem informados.
	AddImage interface {
		Handle(ctx context.Context, uid string, data []byte) (catalog.Product, error)
	}
	RemoveImage interface {
		Handle(ctx context.Context, uid, key string) (catalog.Product, error)
	}
	ReorderImages interface {
		Handle(ctx context.Context, uid string, keys []string) (catalog.Product, error)
	}
}

type adminCatalogHandler struct {
	AdminCatalog
	url urlFunc
}

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
		items = append(items, toResponse(p, h.url))
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
	c.JSON(nethttp.StatusCreated, toResponse(p, h.url))
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
	c.JSON(nethttp.StatusOK, toResponse(p, h.url))
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

// UploadImage recebe a foto (multipart, campo "image"). O tamanho do corpo é limitado ANTES de ler,
// e quem decide se é mesmo uma imagem é o conteúdo do arquivo, não o nome nem o Content-Type enviados.
func (h adminCatalogHandler) UploadImage(c *gin.Context) {
	c.Request.Body = nethttp.MaxBytesReader(c.Writer, c.Request.Body, media.MaxBytes+(1<<20))

	fh, err := c.FormFile("image")
	if err != nil {
		httperr.ErrorHandler(httperr.BadRequestError(`envie a foto no campo "image" (multipart/form-data, até 5 MB)`), c)
		return
	}
	f, err := fh.Open()
	if err != nil {
		httperr.ErrorHandler(httperr.BadRequestError("não consegui ler o arquivo enviado"), c)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, media.MaxBytes+1))
	if err != nil {
		httperr.ErrorHandler(httperr.BadRequestError("não consegui ler o arquivo enviado"), c)
		return
	}

	p, err := h.AdminCatalog.AddImage.Handle(c.Request.Context(), c.Param("uid"), data)
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toResponse(p, h.url))
}

func (h adminCatalogHandler) RemoveImage(c *gin.Context) {
	p, err := h.AdminCatalog.RemoveImage.Handle(c.Request.Context(), c.Param("uid"), c.Param("key"))
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toResponse(p, h.url))
}

type reorderImagesRequest struct {
	Keys []string `json:"keys" binding:"required"`
}

// ReorderImages recebe a ordem completa das fotos; a primeira vira a capa.
func (h adminCatalogHandler) ReorderImages(c *gin.Context) {
	var req reorderImagesRequest
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.AdminCatalog.ReorderImages.Handle(c.Request.Context(), c.Param("uid"), req.Keys)
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toResponse(p, h.url))
}
