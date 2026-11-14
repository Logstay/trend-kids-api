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

// AdminOrders liga as rotas /v1/admin/orders (exigem sessão).
type AdminOrders struct {
	List interface {
		Handle(ctx context.Context, f ports.ListFilter) ([]orders.Order, error)
	}
	Get interface {
		Handle(ctx context.Context, code string) (ports.AdminOrder, error)
	}
	ChangeStatus interface {
		Handle(ctx context.Context, cmd ports.ChangeStatusCommand) (orders.Order, error)
	}
}

type EventResponse struct {
	From    string    `json:"from,omitempty"`
	To      string    `json:"to"`
	AdminID string    `json:"admin_id,omitempty"`
	At      time.Time `json:"at"`
}

// AdminOrderResponse é o pedido para o painel: inclui o telefone, que a loja precisa para falar com o cliente.
type AdminOrderResponse struct {
	OrderResponse
	CustomerPhone string          `json:"customer_phone"`
	Events        []EventResponse `json:"events,omitempty"`
}

type AdminOrderListResponse struct {
	Items []AdminOrderResponse `json:"items"`
}

type changeStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func toAdminOrderResponse(o orders.Order, events []ports.StatusEvent) AdminOrderResponse {
	res := AdminOrderResponse{OrderResponse: toOrderResponse(o), CustomerPhone: o.CustomerPhone()}
	for _, ev := range events {
		res.Events = append(res.Events, EventResponse{From: string(ev.From), To: string(ev.To), AdminID: ev.AdminUID, At: ev.At})
	}
	return res
}

type adminOrdersHandler struct{ AdminOrders }

func (h adminOrdersHandler) List(c *gin.Context) {
	f := ports.ListFilter{Status: c.Query("status")}
	var err error
	if f.Limit, err = intParam(c, "limit"); err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	if f.Offset, err = intParam(c, "offset"); err != nil {
		httperr.ErrorHandler(err, c)
		return
	}

	list, err := h.AdminOrders.List.Handle(c.Request.Context(), f)
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	items := make([]AdminOrderResponse, 0, len(list))
	for _, o := range list {
		items = append(items, toAdminOrderResponse(o, nil))
	}
	c.JSON(nethttp.StatusOK, AdminOrderListResponse{Items: items})
}

func (h adminOrdersHandler) Get(c *gin.Context) {
	view, err := h.AdminOrders.Get.Handle(c.Request.Context(), c.Param("code"))
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toAdminOrderResponse(view.Order, view.Events))
}

func (h adminOrdersHandler) ChangeStatus(c *gin.Context) {
	var req changeStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	o, err := h.AdminOrders.ChangeStatus.Handle(c.Request.Context(), ports.ChangeStatusCommand{
		Code: c.Param("code"), Status: req.Status,
		AdminUID: c.GetString(adminIDKey), // sempre da sessão, nunca do corpo
	})
	if err != nil {
		httperr.ErrorHandler(err, c)
		return
	}
	c.JSON(nethttp.StatusOK, toAdminOrderResponse(o, nil))
}
