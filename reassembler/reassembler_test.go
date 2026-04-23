package reassembler

import (
	"testing"

	"github.com/example/docupolisher/parser"
)

func TestReassembleFallsBackBrokenTextChunk(t *testing.T) {
	placeholder := "[[BLOCK_UUID_11111111-1111-4111-8111-111111111111]]"
	extraction := &parser.ExtractionResult{
		Protected: map[string]string{
			placeholder: "```go\r\nfmt.Println(1)\r\n```",
		},
	}

	original := "Intro " + placeholder + "\r\n\r\nOutro"
	rewritten := "Intro polished\r\n\r\nOutro polished"

	got := Reassemble(original, rewritten, extraction)
	want := "Intro ```go\r\nfmt.Println(1)\r\n```\r\n\r\nOutro polished"
	if got != want {
		t.Fatalf("reassembled markdown mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestReassembleFallsBackWholeDocumentWhenLayoutChanges(t *testing.T) {
	placeholder := "[[BLOCK_UUID_22222222-2222-4222-8222-222222222222]]"
	extraction := &parser.ExtractionResult{
		Protected: map[string]string{
			placeholder: "`code`",
		},
	}

	original := "One " + placeholder + "\n\nTwo"
	rewritten := "One polished\nTwo polished"

	got := Reassemble(original, rewritten, extraction)
	want := "One `code`\n\nTwo"
	if got != want {
		t.Fatalf("reassembled markdown mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestReassembleFallsBackOnUnknownPlaceholder(t *testing.T) {
	known := "[[BLOCK_UUID_33333333-3333-4333-8333-333333333333]]"
	unknown := "[[BLOCK_UUID_44444444-4444-4444-8444-444444444444]]"
	extraction := &parser.ExtractionResult{
		Protected: map[string]string{
			known: "`code`",
		},
	}

	original := "Known " + known + "\n"
	rewritten := "Known " + known + "\n\nUnexpected " + unknown

	got := Reassemble(original, rewritten, extraction)
	want := "Known `code`\n"
	if got != want {
		t.Fatalf("reassembled markdown mismatch\nwant: %q\ngot:  %q", want, got)
	}
}
