package router

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	apperrors "tsuruyomi/internal/errors"
	"tsuruyomi/internal/util"
)

// CentralErrorHandler translates domain AppError and Fiber errors into protocol-compliant responses
// across REST v1, GraphQL, and OPDS feeds.
func CentralErrorHandler(c *fiber.Ctx, err error) error {
	var appErr *apperrors.AppError
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	message := "An internal server error occurred"

	if errors.As(err, &appErr) {
		message = appErr.Message
		code = appErr.Code
		switch appErr.Kind {
		case apperrors.KindNotFound:
			status = http.StatusNotFound
		case apperrors.KindInvalidInput:
			status = http.StatusBadRequest
		case apperrors.KindUnauthorized:
			status = http.StatusUnauthorized
		case apperrors.KindForbidden:
			status = http.StatusForbidden
		case apperrors.KindConflict:
			status = http.StatusConflict
		case apperrors.KindUpstreamFailed:
			status = http.StatusBadGateway
		case apperrors.KindServiceUnavailable:
			status = http.StatusServiceUnavailable
		}
	} else {
		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			status = fiberErr.Code
			message = fiberErr.Message
			code = strings.ToUpper(strings.ReplaceAll(http.StatusText(status), " ", "_"))
		}
	}

	logger := util.LoggerFromContext(c.UserContext())
	if status >= 500 {
		logger.Error("HTTP request failed with server error",
			slog.Int("status", status),
			slog.String("path", c.Path()),
			slog.String("code", code),
			slog.String("message", message),
			slog.Any("err", err),
		)
	} else {
		logger.Debug("HTTP request completed with client error",
			slog.Int("status", status),
			slog.String("path", c.Path()),
			slog.String("code", code),
			slog.String("message", message),
		)
	}

	path := c.Path()
	if strings.HasPrefix(path, "/api/graphql") {
		return c.Status(http.StatusOK).JSON(fiber.Map{
			"errors": []fiber.Map{
				{
					"message": message,
					"extensions": fiber.Map{
						"code": code,
					},
				},
			},
		})
	}

	if strings.HasPrefix(path, "/opds") || strings.HasPrefix(path, "/api/opds") {
		return c.Status(status).SendString(message)
	}

	return c.Status(status).JSON(fiber.Map{
		"error": message,
		"code":  code,
	})
}
