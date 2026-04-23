package parser

import (
	"crypto/rand"
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type ProtectedKind string

const (
	ProtectedFencedCode ProtectedKind = "fenced_code"
	ProtectedMathBlock  ProtectedKind = "math_block"
	ProtectedInlineMath ProtectedKind = "inline_math"
)

type ProtectedBlock struct {
	ID          string
	Placeholder string
	Kind        ProtectedKind
	Original    string
	Start       int
	End         int
}

type ExtractionResult struct {
	Markdown  string
	Protected map[string]string
	Blocks    []ProtectedBlock
}

type Extractor struct {
	md goldmark.Markdown
}

func NewExtractor() *Extractor {
	return &Extractor{
		md: goldmark.New(goldmark.WithExtensions(MathExtension{})),
	}
}

func Extract(markdown string) (*ExtractionResult, error) {
	return NewExtractor().Extract(markdown)
}

func (e *Extractor) Extract(markdown string) (*ExtractionResult, error) {
	source := []byte(markdown)
	doc := e.md.Parser().Parse(text.NewReader(source))

	ranges := make([]protectedRange, 0)
	err := gmast.Walk(doc, func(node gmast.Node, entering bool) (gmast.WalkStatus, error) {
		if !entering {
			return gmast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *gmast.FencedCodeBlock:
			r, ok := fencedCodeRange(source, n)
			if ok {
				ranges = append(ranges, protectedRange{Kind: ProtectedFencedCode, Start: r.Start, End: r.End})
				return gmast.WalkSkipChildren, nil
			}
		case *MathBlock:
			r, ok := mathBlockRange(source, n)
			if ok {
				ranges = append(ranges, protectedRange{Kind: ProtectedMathBlock, Start: r.Start, End: r.End})
				return gmast.WalkSkipChildren, nil
			}
		case *InlineMath:
			r, ok := inlineMathRange(source, n)
			if ok {
				ranges = append(ranges, protectedRange{Kind: ProtectedInlineMath, Start: r.Start, End: r.End})
			}
		}
		return gmast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}

	return replaceProtectedRanges(markdown, ranges)
}

type protectedRange struct {
	Kind  ProtectedKind
	Start int
	End   int
}

func replaceProtectedRanges(source string, ranges []protectedRange) (*ExtractionResult, error) {
	ranges = normalizeRanges(len(source), ranges)

	protected := make(map[string]string, len(ranges))
	blocks := make([]ProtectedBlock, 0, len(ranges))
	builder := []byte(source)

	for i := len(ranges) - 1; i >= 0; i-- {
		r := ranges[i]
		id, placeholder, err := newPlaceholder()
		if err != nil {
			return nil, err
		}

		original := source[r.Start:r.End]
		protected[placeholder] = original
		blocks = append(blocks, ProtectedBlock{
			ID:          id,
			Placeholder: placeholder,
			Kind:        r.Kind,
			Original:    original,
			Start:       r.Start,
			End:         r.End,
		})

		next := make([]byte, 0, len(builder)-(r.End-r.Start)+len(placeholder))
		next = append(next, builder[:r.Start]...)
		next = append(next, placeholder...)
		next = append(next, builder[r.End:]...)
		builder = next
	}

	reverseBlocks(blocks)
	return &ExtractionResult{
		Markdown:  string(builder),
		Protected: protected,
		Blocks:    blocks,
	}, nil
}

func normalizeRanges(sourceLen int, ranges []protectedRange) []protectedRange {
	valid := ranges[:0]
	for _, r := range ranges {
		if r.Start < 0 || r.End > sourceLen || r.Start >= r.End {
			continue
		}
		valid = append(valid, r)
	}
	sort.SliceStable(valid, func(i, j int) bool {
		if valid[i].Start == valid[j].Start {
			return valid[i].End > valid[j].End
		}
		return valid[i].Start < valid[j].Start
	})

	compacted := valid[:0]
	lastEnd := -1
	for _, r := range valid {
		if r.Start < lastEnd {
			continue
		}
		compacted = append(compacted, r)
		lastEnd = r.End
	}
	return compacted
}

func reverseBlocks(blocks []ProtectedBlock) {
	for i, j := 0, len(blocks)-1; i < j; i, j = i+1, j-1 {
		blocks[i], blocks[j] = blocks[j], blocks[i]
	}
}

func newPlaceholder() (string, string, error) {
	id, err := uuidV4()
	if err != nil {
		return "", "", err
	}
	return id, "[[BLOCK_UUID_" + id + "]]", nil
}

func uuidV4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	), nil
}

type byteRange struct {
	Start int
	End   int
}

func fencedCodeRange(source []byte, node *gmast.FencedCodeBlock) (byteRange, bool) {
	start := node.Pos()
	if start < 0 || start >= len(source) {
		return byteRange{}, false
	}

	lineEnd := lineContentEnd(source, start)
	if start >= lineEnd {
		return byteRange{}, false
	}

	fenceChar := source[start]
	if fenceChar != '`' && fenceChar != '~' {
		return byteRange{}, false
	}

	fenceLen := 0
	for i := start; i < lineEnd && source[i] == fenceChar; i++ {
		fenceLen++
	}
	if fenceLen < 3 {
		return byteRange{}, false
	}

	for lineStart := nextLineStart(source, start); lineStart < len(source); lineStart = nextLineStart(source, lineStart) {
		contentEnd := lineContentEnd(source, lineStart)
		pos, width := firstNonSpace(source, lineStart, contentEnd)
		if width < 4 && pos < contentEnd && source[pos] == fenceChar {
			count := repeatedByteCount(source, pos, contentEnd, fenceChar)
			if count >= fenceLen && isBlankBytes(source[pos+count:contentEnd]) {
				return byteRange{Start: start, End: contentEnd}, true
			}
		}
		next := nextLineStart(source, lineStart)
		if next <= lineStart {
			break
		}
	}

	return byteRange{Start: start, End: len(source)}, true
}

func mathBlockRange(source []byte, node *MathBlock) (byteRange, bool) {
	if node.StartOffset < 0 || node.StartOffset >= len(source) {
		return byteRange{}, false
	}
	stop := node.StopOffset
	if stop < node.StartOffset || stop > len(source) {
		stop = len(source)
	}
	return byteRange{Start: node.StartOffset, End: stop}, true
}

func inlineMathRange(source []byte, node *InlineMath) (byteRange, bool) {
	start := node.Segment.Start
	stop := node.Segment.Stop
	if start < 0 || stop > len(source) || start >= stop {
		return byteRange{}, false
	}
	return byteRange{Start: start, End: stop}, true
}

func absoluteOffset(segment text.Segment, localIndex int) int {
	return segment.Start - segment.Padding + localIndex
}

func lineContentEnd(source []byte, lineStart int) int {
	if lineStart < 0 {
		return 0
	}
	if lineStart > len(source) {
		return len(source)
	}
	newline := strings.IndexByte(string(source[lineStart:]), '\n')
	end := len(source)
	if newline >= 0 {
		end = lineStart + newline
	}
	if end > lineStart && source[end-1] == '\r' {
		end--
	}
	return end
}

func nextLineStart(source []byte, lineStart int) int {
	if lineStart < 0 {
		return 0
	}
	if lineStart >= len(source) {
		return len(source)
	}
	newline := strings.IndexByte(string(source[lineStart:]), '\n')
	if newline < 0 {
		return len(source)
	}
	return lineStart + newline + 1
}

func firstNonSpace(source []byte, start, end int) (int, int) {
	width := 0
	for i := start; i < end; i++ {
		switch source[i] {
		case ' ':
			width++
		case '\t':
			width += 4 - width%4
		default:
			return i, width
		}
	}
	return end, width
}

func repeatedByteCount(source []byte, start, end int, want byte) int {
	count := 0
	for start+count < end && source[start+count] == want {
		count++
	}
	return count
}

func isBlankBytes(source []byte) bool {
	for _, b := range source {
		if b != ' ' && b != '\t' {
			return false
		}
	}
	return true
}
