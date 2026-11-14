package http

import (
	"context"
	nethttp "net/http"
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/gin-gonic/gin"
)

// Orders liga as rotas públicas de pedido.
type Orders struct {
	Place interface {
		Handle(ctx context.Context, cmd ports.PlaceOrder) (orders.Order, error)
	}
	Lookup interface {
		Handle(ctx context.Context, q ports.Lookup) (orders.Order, error)
	}
}

type placeOrderRequest struct {
	Customer struct {
		Name         string `json:"name" binding:"required"`
		Phone        string `json:"phone" binding:"required"`
		Neighborhood string `json:"neighborhood" binding:"required"`
	} `json:"customer" binding:"required"`
	// Sem preço, de propósito: o servidor sempre usa o preço do catálogo.
	Items []struct {
		ProductID string `json:"product_id" binding:"required"`
		Quantity  int    `json:"quantity" binding:"required,min=1,max=20"`
	} `json:"items" binding:"required,min=1,max=20,dive"`
}

type lookupRequest struct {
	Code  string `json:"code" binding:"required"`
	Phone string `json:"phone" binding:"required"`
}

type OrderItemResponse struct {
	ProductID      string `json:"product_id"`
	Name           string `json:"name"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	Quantity       int    `json:"quantity"`
	TotalCents     int64  `json:"total_cents"`
}

// OrderResponse não inclui o telefone: quem consulta já o informou.
type OrderResponse struct {
	Code          string              `json:"code"`
	Status        string              `json:"status"`
	CustomerName  string              `json:"customer_name"`
	Neighborhood  string              `json:"neighborhood"`
	Items         []OrderItemResponse `json:"items"`
	SubtotalCents int64               `json:"subtotal_cents"`
	ShippingCents int64               `json:"shipping_cents"`
	TotalCents    int64               `json:"total_cents"`
	CreatedAt     time.Time           `json:"created_at"`
}

func toOrderResponse(o orders.Order) OrderResponse {
	items := make([]OrderItemResponse, 0, len(o.Items()))
	for _, it := range o.Items() {
		items = append(items, OrderItemResponse{
			ProductID: it.ProductUID, Name: it.Name, UnitPriceCents: it.UnitPriceCents,
			Quantity: it.Quantity, TotalCents: it.TotalCents(),
		})
	}
	return OrderResponse{
		Code: o.Code(), Status: string(o.Status()), CustomerName: o.CustomerName(), Neighborhood: o.Neighborhood(),
		Items: items, SubtotalCents: o.SubtotalCents(), ShippingCents: o.ShippingCents(), TotalCents: o.TotalCents(),
		CreatedAt: o.CreatedAt(),
	}
}

type ordersHandler struct{ Orders }

func (h ordersHandler) PlaceOrder(c *gin.Context) {
	var req placeOrderRequest
	if !bindJSON(c, &req) {
		return
	}
	cmd := ports.PlaceOrder{
		CustomerName: req.Customer.Name, Phone: req.Customer.Phone, Neighborhood: req.Customer.Neighborhood,
	}
	for _, it := range req.Items {
		cmd.Items = append(cmd.Items, ports.PlaceItem{ProductUID: it.ProductID, Quantity: it.Quantity})
	}

	o, err := h.Place.Handle(c.Request.Context(), cmd)
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusCreated, toOrderResponse(o))
}

// LookupOrder é POST (e não GET com ?phone=) para o telefone não ir parar em URL nem em log de acesso.
func (h ordersHandler) LookupOrder(c *gin.Context) {
	var req lookupRequest
	if !bindJSON(c, &req) {
		return
	}
	o, err := h.Lookup.Handle(c.Request.Context(), ports.Lookup{Code: req.Code, Phone: req.Phone})
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toOrderResponse(o))
}
