package mcp

import (
	"errors"
	"testing"

	"github.com/bulislaw/suuntool/internal/api"
)

func TestMapError_TypedAPIError(t *testing.T) {
	e := &api.Error{Code: "AUTH_EXPIRED", Message: "session expired", Hint: "Run: suuntool login", HTTP: 401, Exit: 4}
	res := mapError(e)
	if !res.IsError {
		t.Fatal("expected IsError=true")
	}
	if res.StructuredContent["code"] != "AUTH_EXPIRED" {
		t.Fatalf("expected code AUTH_EXPIRED, got %v", res.StructuredContent["code"])
	}
}

func TestMapError_Plain(t *testing.T) {
	res := mapError(errors.New("boom"))
	if !res.IsError {
		t.Fatal("expected IsError=true")
	}
	if res.StructuredContent["code"] != "UNKNOWN" {
		t.Fatalf("expected code UNKNOWN, got %v", res.StructuredContent["code"])
	}
}

func TestMapErrorToCallToolResult(t *testing.T) {
	e := &api.Error{Code: "AUTH_EXPIRED", Message: "x"}
	res := mapErrorToCallToolResult(e)
	if !res.IsError {
		t.Fatal("expected IsError")
	}
	if len(res.Content) == 0 {
		t.Fatal("expected Content to be populated")
	}
	if res.StructuredContent == nil {
		t.Fatal("expected StructuredContent to be populated")
	}
}
