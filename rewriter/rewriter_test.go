package rewriter

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestProcessDocumentPreservesSeparators(t *testing.T) {
	var calls []string
	rw := &Rewriter{
		config: Config{Concurrency: 1},
		rewriteText: func(ctx context.Context, text string) (string, error) {
			calls = append(calls, text)
			return strings.ToUpper(text), nil
		},
	}

	input := "\r\n\r\nfirst\r\n\r\nsecond line\r\nstill second\r\n"
	got, err := rw.ProcessDocument(context.Background(), input)
	if err != nil {
		t.Fatalf("ProcessDocument: %v", err)
	}

	want := "\r\n\r\nFIRST\r\n\r\nSECOND LINE\r\nSTILL SECOND\r\n"
	if got != want {
		t.Fatalf("rewritten markdown mismatch\nwant: %q\ngot:  %q", want, got)
	}

	wantCalls := []string{
		"first",
		"second line\r\nstill second",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("rewrite calls mismatch\nwant: %#v\ngot:  %#v", wantCalls, calls)
	}
}
