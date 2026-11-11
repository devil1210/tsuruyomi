package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestAppError_Formatting(t *testing.T) {
	errWithoutCause := NotFound("MANGA_NOT_FOUND", "manga ID 42 was not found", nil)
	expectedWithout := "[MANGA_NOT_FOUND] manga ID 42 was not found"
	if errWithoutCause.Error() != expectedWithout {
		t.Errorf("expected %q, got %q", expectedWithout, errWithoutCause.Error())
	}

	rootErr := errors.New("connection reset by peer")
	errWithCause := UpstreamFailed("SCRAPER_TIMEOUT", "failed fetching chapter list", rootErr)
	expectedWith := "[SCRAPER_TIMEOUT] failed fetching chapter list: connection reset by peer"
	if errWithCause.Error() != expectedWith {
		t.Errorf("expected %q, got %q", expectedWith, errWithCause.Error())
	}
}

func TestAppError_Unwrap(t *testing.T) {
	root := errors.New("sql: no rows in result set")
	wrapped := NotFound("CATEGORY_NOT_FOUND", "category not found", root)

	if !errors.Is(wrapped, root) {
		t.Errorf("expected errors.Is to match unwrapped root cause")
	}

	unwrapped := errors.Unwrap(wrapped)
	if unwrapped != root {
		t.Errorf("expected %v, got %v", root, unwrapped)
	}
}

func TestAppError_Is(t *testing.T) {
	err1 := NotFound("MANGA_NOT_FOUND", "not found", nil)
	targetSameKindSameCode := &AppError{Kind: KindNotFound, Code: "MANGA_NOT_FOUND"}
	targetSameKindEmptyCode := &AppError{Kind: KindNotFound}
	targetSameKindDiffCode := &AppError{Kind: KindNotFound, Code: "CHAPTER_NOT_FOUND"}
	targetDiffKind := &AppError{Kind: KindInvalidInput, Code: "MANGA_NOT_FOUND"}

	if !errors.Is(err1, targetSameKindSameCode) {
		t.Errorf("expected errors.Is to match same kind and code")
	}
	if !errors.Is(err1, targetSameKindEmptyCode) {
		t.Errorf("expected errors.Is to match same kind when target code is empty")
	}
	if errors.Is(err1, targetSameKindDiffCode) {
		t.Errorf("expected errors.Is to NOT match when code differs")
	}
	if errors.Is(err1, targetDiffKind) {
		t.Errorf("expected errors.Is to NOT match when kind differs")
	}
	if errors.Is(err1, errors.New("generic error")) {
		t.Errorf("expected errors.Is to NOT match non-AppError")
	}
}

func TestAppError_As(t *testing.T) {
	original := InvalidInput("BAD_PAGE_INDEX", "page index must be >= 0", nil)
	wrapped := fmt.Errorf("context wrapper: %w", original)

	var appErr *AppError
	if !errors.As(wrapped, &appErr) {
		t.Fatalf("expected errors.As to extract *AppError")
	}
	if appErr.Code != "BAD_PAGE_INDEX" {
		t.Errorf("expected code BAD_PAGE_INDEX, got %q", appErr.Code)
	}
	if appErr.Kind != KindInvalidInput {
		t.Errorf("expected kind KindInvalidInput, got %v", appErr.Kind)
	}
}

func TestAppError_Constructors(t *testing.T) {
	cases := []struct {
		name     string
		err      *AppError
		wantKind Kind
		wantCode string
	}{
		{"Internal", Internal("SYS_ERR", "internal error", nil), KindInternal, "SYS_ERR"},
		{"NotFound", NotFound("NOT_FOUND", "not found", nil), KindNotFound, "NOT_FOUND"},
		{"InvalidInput", InvalidInput("BAD_REQ", "bad req", nil), KindInvalidInput, "BAD_REQ"},
		{"Unauthorized", Unauthorized("AUTH_REQUIRED", "unauthorized", nil), KindUnauthorized, "AUTH_REQUIRED"},
		{"Forbidden", Forbidden("ACCESS_DENIED", "forbidden", nil), KindForbidden, "ACCESS_DENIED"},
		{"Conflict", Conflict("SLUG_EXISTS", "conflict", nil), KindConflict, "SLUG_EXISTS"},
		{"UpstreamFailed", UpstreamFailed("UPSTREAM_FAIL", "upstream failed", nil), KindUpstreamFailed, "UPSTREAM_FAIL"},
		{"ServiceUnavailable", ServiceUnavailable("MAINTENANCE", "service down", nil), KindServiceUnavailable, "MAINTENANCE"},
		{"NewCustom", New(KindForbidden, "FORBIDDEN_CUSTOM", "custom", nil), KindForbidden, "FORBIDDEN_CUSTOM"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Kind != tc.wantKind {
				t.Errorf("kind mismatch: got %v, want %v", tc.err.Kind, tc.wantKind)
			}
			if tc.err.Code != tc.wantCode {
				t.Errorf("code mismatch: got %q, want %q", tc.err.Code, tc.wantCode)
			}
			if tc.err.Kind.String() == "Unknown" {
				t.Errorf("Kind.String() returned Unknown for %v", tc.err.Kind)
			}
		})
	}
}
