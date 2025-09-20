package dto

import (
	"errors"
	"testing"
	"time"
)

func TestErrorResponse_Error(t *testing.T) {
	e := ErrorResponse{Message: "oops"}
	if e.Error() != "oops" {
		t.Fatalf("want 'oops' got %q", e.Error())
	}
	e2 := ErrorResponse{Message: "oops", ErrorDetails: "bad"}
	if e2.Error() != "oops: bad" {
		t.Fatalf("want 'oops: bad' got %q", e2.Error())
	}
}

func TestNewErrorResponse(t *testing.T) {
	// without inner error
	e := NewErrorResponse("msg", nil)
	if e.Message != "msg" || e.ErrorDetails != "" {
		t.Fatalf("unexpected %+v", e)
	}
	if e.Timestamp.IsZero() || time.Since(e.Timestamp) > time.Second {
		t.Fatalf("timestamp not set")
	}

	// with inner error
	err := errors.New("boom")
	e2 := NewErrorResponse("msg", err)
	if e2.ErrorDetails != "boom" || e2.Message != "msg" {
		t.Fatalf("unexpected %+v", e2)
	}
}
