package utils

import (
	"github.com/labstack/echo/v4"
)

type HTTPErrorResponse struct {
	Error        error  `json:"error"`
	Message      string `json:"message"`
	ErrorMessage string `json:"errorMessage"`
}

func HTTPFail(ctx echo.Context, code int, err error, message string) error {
	result := &HTTPErrorResponse{
		Error:   err,
		Message: message,
	}

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	return ctx.JSON(code, result)
}
