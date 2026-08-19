package utils

import (
	"github.com/labstack/echo/v4"
)

type HTTPErrorResponse struct {
	// Error is a legacy opaque value. Go's JSON encoder does not serialize an
	// error message here, so clients should use Message and ErrorMessage.
	Error        error  `json:"error" swaggertype:"object"`
	Message      string `json:"message"`
	ErrorMessage string `json:"errorMessage"`
}

// AuthenticationErrorResponse is Echo's public error body when API key
// authentication fails before a controller is reached.
type AuthenticationErrorResponse struct {
	Message string `json:"message" example:"Unauthorized"`
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
