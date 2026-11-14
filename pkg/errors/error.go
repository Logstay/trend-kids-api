package errors

import (
	goerrors "errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

// ErrorHandler traduz um erro tipado (com StatusCode) em resposta JSON.
// Erro desconhecido vira 500 genérico, sem vazar detalhe interno.
func ErrorHandler(err error, c *gin.Context) {
	type statusCoder interface{ StatusCode() int }

	res := ErrorResponse{Code: http.StatusInternalServerError, Error: "erro interno"}
	var sc statusCoder
	if goerrors.As(err, &sc) {
		res = ErrorResponse{Code: sc.StatusCode(), Error: err.Error()}
	}
	c.AbortWithStatusJSON(res.Code, res)
}
