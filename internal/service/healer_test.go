package service

import (
	"context"
	"testing"
)

func TestHealer_NoDB(t *testing.T) {
	// When database is not initialized (DB == nil), Audit should return an error
	_, err := Audit(context.Background())
	if err == nil {
		t.Fatalf("expected error when DB is nil, got nil")
	}

	_, errHeal := Heal(context.Background())
	if errHeal == nil {
		t.Fatalf("expected error when DB is nil, got nil")
	}
}
