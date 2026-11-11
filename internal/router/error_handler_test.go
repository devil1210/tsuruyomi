package router

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	apperrors "tsuruyomi/internal/errors"
)

func setupTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: CentralErrorHandler,
	})

	app.Get("/api/v1/manga/:id", func(c *fiber.Ctx) error {
		return apperrors.NotFound("MANGA_NOT_FOUND", "manga 123 not found", nil)
	})

	app.Get("/api/v1/invalid", func(c *fiber.Ctx) error {
		return apperrors.InvalidInput("INVALID_PARAM", "param id must be positive", nil)
	})

	app.Get("/api/v1/fiber-err", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusPaymentRequired, "Payment Required")
	})

	app.Get("/api/v1/panic-err", func(c *fiber.Ctx) error {
		return errors.New("raw unexpected failure")
	})

	app.Post("/api/graphql", func(c *fiber.Ctx) error {
		return apperrors.UpstreamFailed("RESOLVER_FAILURE", "cannot resolve field manga", nil)
	})

	app.Get("/opds/v1.2/catalog", func(c *fiber.Ctx) error {
		return apperrors.NotFound("CATALOG_NOT_FOUND", "feed does not exist", nil)
	})

	app.Get("/api/opds/2.0/feed", func(c *fiber.Ctx) error {
		return apperrors.Unauthorized("AUTH_REQUIRED", "opds credentials missing", nil)
	})

	return app
}

func TestCentralErrorHandler_REST_AppError(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manga/123", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var res map[string]string
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("failed to decode json body: %v", err)
	}

	if res["code"] != "MANGA_NOT_FOUND" {
		t.Errorf("expected code MANGA_NOT_FOUND, got %q", res["code"])
	}
	if res["error"] != "manga 123 not found" {
		t.Errorf("expected error 'manga 123 not found', got %q", res["error"])
	}
}

func TestCentralErrorHandler_REST_FiberError(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/fiber-err", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusPaymentRequired {
		t.Errorf("expected status %d, got %d", fiber.StatusPaymentRequired, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var res map[string]string
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("failed to decode json body: %v", err)
	}

	if res["code"] != "PAYMENT_REQUIRED" {
		t.Errorf("expected code PAYMENT_REQUIRED, got %q", res["code"])
	}
	if res["error"] != "Payment Required" {
		t.Errorf("expected error 'Payment Required', got %q", res["error"])
	}
}

func TestCentralErrorHandler_GraphQL(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest(http.MethodPost, "/api/graphql", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// GraphQL errors must return HTTP 200 with errors payload
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK for GraphQL error, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		Errors []struct {
			Message    string                 `json:"message"`
			Extensions map[string]interface{} `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("failed to decode graphql response: %v", err)
	}

	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 error in graphql response, got %d", len(res.Errors))
	}
	if res.Errors[0].Message != "cannot resolve field manga" {
		t.Errorf("unexpected error message: %q", res.Errors[0].Message)
	}
	if res.Errors[0].Extensions["code"] != "RESOLVER_FAILURE" {
		t.Errorf("expected extension code RESOLVER_FAILURE, got %v", res.Errors[0].Extensions["code"])
	}
}

func TestCentralErrorHandler_OPDS(t *testing.T) {
	app := setupTestApp()

	// OPDS 1.2
	req1 := httptest.NewRequest(http.MethodGet, "/opds/v1.2/catalog", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp1.StatusCode)
	}
	body1, _ := io.ReadAll(resp1.Body)
	if string(body1) != "feed does not exist" {
		t.Errorf("expected body 'feed does not exist', got %q", string(body1))
	}

	// OPDS 2.0
	req2 := httptest.NewRequest(http.MethodGet, "/api/opds/2.0/feed", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("request 2 failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp2.StatusCode)
	}
	body2, _ := io.ReadAll(resp2.Body)
	if string(body2) != "opds credentials missing" {
		t.Errorf("expected body 'opds credentials missing', got %q", string(body2))
	}
}

func TestCentralErrorHandler_InternalErrorFallback(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/panic-err", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var res map[string]string
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["code"] != "INTERNAL_ERROR" {
		t.Errorf("expected code INTERNAL_ERROR, got %q", res["code"])
	}
}
