package router

import (
	"testing"

	"github.com/Dishank-Sen/quicnode/internal/stream"
)

func TestStreamRoute_Valid(t *testing.T) {
	r := NewRouter()
	handler := func(ctx stream.Context) {}
	r.StreamRoute("ping", handler)
}

func TestStreamRoute_NilHandler(t *testing.T) {
    r := NewRouter()

    defer func() {
        if rec := recover(); rec == nil {
            t.Error("expected panic for nil handler, but did not panic")
        }
    }()

    r.StreamRoute("ping", nil) // should panic
}

func TestStreamRoute_EmptyRoute(t *testing.T) {
    r := NewRouter()

    defer func() {
        if rec := recover(); rec == nil {
            t.Error("expected panic for nil handler, but did not panic")
        }
    }()
	handler := func(ctx stream.Context) {}
    r.StreamRoute("", handler) // should panic
}

func TestGetHandler_ExistingRoute(t *testing.T) {
    r := NewRouter()
    handler := func(ctx stream.Context) {}
    r.StreamRoute("ping", handler)

    h, ok := r.GetStreamHandler("ping")
    if !ok {
        t.Fatal("expected handler to exist, got false")
    }
    if h == nil {
        t.Error("expected non-nil handler, got nil")
    }
}

func TestGetHandler_NonExistingRoute(t *testing.T) {
    r := NewRouter()

    h, ok := r.GetStreamHandler("ping")
    if ok {
        t.Error("expected false for non-existing route, got true")
    }
    if h != nil {
        t.Error("expected nil handler for non-existing route")
    }
}