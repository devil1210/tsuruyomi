package util

import (
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Name", "Normal Name"},
		{"Manga: Subtitle / Title? * < > |", "Manga_ Subtitle _ Title_ _ _ _ _"},
		{"...dots and spaces... ", "dots and spaces"},
		{"", "(invalid)"},
		{"Japanese: 鶴読み - Tsuruyomi", "Japanese_ 鶴読み - Tsuruyomi"},
	}

	for _, tt := range tests {
		actual := SanitizeFilename(tt.input)
		if actual != tt.expected {
			t.Errorf("SanitizeFilename(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestDetectImageType(t *testing.T) {
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	webpHeader := []byte("RIFFxxxxWEBP")
	gifHeader := []byte("GIF89a")

	if mime := DetectImageType(jpegHeader); mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
	if mime := DetectImageType(pngHeader); mime != "image/png" {
		t.Errorf("expected image/png, got %s", mime)
	}
	if mime := DetectImageType(webpHeader); mime != "image/webp" {
		t.Errorf("expected image/webp, got %s", mime)
	}
	if mime := DetectImageType(gifHeader); mime != "image/gif" {
		t.Errorf("expected image/gif, got %s", mime)
	}
}
