package markdownutil

import (
	"reflect"
	"testing"
)

func TestSplitChunksPreservesCRLFLayout(t *testing.T) {
	input := "\r\n\r\nalpha\r\n\r\n  beta\r\ngamma\r\n \r\n\r\ndelta\r\n"

	chunks := SplitChunks(input)
	if got := JoinChunks(chunks); got != input {
		t.Fatalf("joined chunks mismatch\nwant: %q\ngot:  %q", input, got)
	}

	wantKinds := []ChunkKind{
		ChunkSeparator,
		ChunkText,
		ChunkSeparator,
		ChunkText,
		ChunkSeparator,
		ChunkText,
		ChunkSeparator,
	}
	gotKinds := make([]ChunkKind, 0, len(chunks))
	for _, chunk := range chunks {
		gotKinds = append(gotKinds, chunk.Kind)
	}
	if !reflect.DeepEqual(gotKinds, wantKinds) {
		t.Fatalf("chunk kinds mismatch\nwant: %#v\ngot:  %#v", wantKinds, gotKinds)
	}

	wantTexts := []string{
		"\r\n\r\n",
		"alpha",
		"\r\n\r\n",
		"  beta\r\ngamma",
		"\r\n \r\n\r\n",
		"delta",
		"\r\n",
	}
	gotTexts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		gotTexts = append(gotTexts, chunk.Text)
	}
	if !reflect.DeepEqual(gotTexts, wantTexts) {
		t.Fatalf("chunk texts mismatch\nwant: %#v\ngot:  %#v", wantTexts, gotTexts)
	}
}
