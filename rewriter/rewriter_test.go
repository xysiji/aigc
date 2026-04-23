package rewriter

import (
	"context"
	"errors"
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

func TestProcessDocumentPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := 0
	rw := &Rewriter{
		config: Config{Concurrency: 1},
		rewriteText: func(ctx context.Context, text string) (string, error) {
			callCount++
			cancel()
			<-ctx.Done()
			return "", ctx.Err()
		},
	}

	_, err := rw.ProcessDocument(ctx, "first\r\n\r\nsecond")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ProcessDocument error = %v, want context.Canceled", err)
	}
	if callCount != 2 {
		t.Fatalf("rewrite call count = %d, want 2", callCount)
	}
}

func TestCleanLLMOutputUnwrapsSingleOuterFence(t *testing.T) {
	input := "```markdown\r\nline 1\r\nline 2\r\n```\r\n"
	want := "line 1\r\nline 2"
	if got := cleanLLMOutput(input); got != want {
		t.Fatalf("cleanLLMOutput mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestCleanLLMOutputPreservesIntentionalTrailingBlankLine(t *testing.T) {
	input := "```\nline 1\nline 2\n\n```\n"
	want := "line 1\nline 2\n"
	if got := cleanLLMOutput(input); got != want {
		t.Fatalf("cleanLLMOutput mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestCleanLLMOutputLeavesNonWrappedTextAlone(t *testing.T) {
	input := "prefix\n```markdown\ntext\n```\n"
	if got := cleanLLMOutput(input); got != input {
		t.Fatalf("cleanLLMOutput changed plain text\nwant: %q\ngot:  %q", input, got)
	}
}
