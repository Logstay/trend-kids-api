package http

import (
	"context"
	nethttp "net/http"
	"strconv"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/gin-gonic/gin"
)

type ListUseCase interface {
	Handle(ctx context.Context, filter ports.Filter) ([]catalog.Product, error)
}

type GetUseCase interface {
	Handle(ctx context.Context, slug string) (catalog.Product, error)
}

type catalogHandler struct {
	list ListUseCase
	get  GetUseCase
}

func (h catalogHandler) List(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}

	products, err := h.list.Handle(c.Request.Context(), filter)
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

func (h catalogHandler) Get(c *gin.Context) {
	p, err := h.get.Handle(c.Request.Context(), c.Param("slug"))
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toResponse(p))
}

func parseFilter(c *gin.Context) (f ports.Filter, err error) {
	f.Category = c.Query("category")
	f.Query = c.Query("q")

	switch s := ports.Sort(c.Query("sort")); s {
	case "", ports.SortNewest, ports.SortPriceAsc, ports.SortPriceDesc:
		f.Sort = s
	default:
		return f, httperr.BadRequestError("sort inválido")
	}

	if f.Limit, err = intParam(c, "limit"); err != nil {
		return f, err
	}
	f.Offset, err = intParam(c, "offset")
	return f, err
}

func intParam(c *gin.Context, name string) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, httperr.BadRequestError(name + " inválido")
	}
	return n, nil
}
