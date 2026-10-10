package sqlite

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func TestErrorClassification(t *testing.T) {
	cases := []struct {
		input, errorClass error
		code              Code
	}{
		{record.ErrCorrupt, ErrCorrupt, CodeCorrupt},
		{record.ErrLimit, ErrMemoryLimit, CodeNoMem},
		{record.ErrUnsupported, ErrUnsupported, CodeError},
		{pager.ErrCorrupt, ErrCorrupt, CodeCorrupt},
		{pager.ErrUnsupported, ErrUnsupported, CodeError},
		{&sqlast.ErrUnsupported{Feature: "test"}, ErrUnsupported, CodeError},
		{ErrReadOnly, ErrReadOnly, CodeReadOnly},
		{context.Canceled, context.Canceled, CodeInterrupt},
		{ErrClosed, ErrClosed, CodeMisuse},
	}
	for _, tc := range cases {
		err := wrap("test", fmt.Errorf("context: %w", tc.input))
		var typed *Error
		if !errors.Is(err, tc.errorClass) || !errors.As(err, &typed) || typed.Code != tc.code {
			t.Fatalf("input %v: %v", tc.input, err)
		}
		if !errors.Is(err, tc.input) {
			t.Fatalf("lost original error: %v", err)
		}
	}
}
