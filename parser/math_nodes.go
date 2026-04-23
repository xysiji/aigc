package parser

import (
	"fmt"

	gmast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var (
	// KindMathBlock marks a display math block parsed by the DocuPolisher
	// goldmark extension.
	KindMathBlock = gmast.NewNodeKind("DocuPolisherMathBlock")

	// KindInlineMath marks inline LaTeX math parsed by the DocuPolisher
	// goldmark extension.
	KindInlineMath = gmast.NewNodeKind("DocuPolisherInlineMath")
)

// MathBlock represents a display LaTeX block delimited by $$...$$ or \[...\].
type MathBlock struct {
	gmast.BaseBlock

	Opener      string
	Closer      string
	StartOffset int
	StopOffset  int
}

func NewMathBlock(opener, closer string, start int) *MathBlock {
	return &MathBlock{
		Opener:      opener,
		Closer:      closer,
		StartOffset: start,
		StopOffset:  -1,
	}
}

func (n *MathBlock) Kind() gmast.NodeKind {
	return KindMathBlock
}

func (n *MathBlock) IsRaw() bool {
	return true
}

func (n *MathBlock) Text(source []byte) []byte {
	if n.StartOffset < 0 || n.StartOffset > len(source) {
		return nil
	}
	stop := n.StopOffset
	if stop < n.StartOffset || stop > len(source) {
		stop = len(source)
	}
	return source[n.StartOffset:stop]
}

func (n *MathBlock) Dump(source []byte, level int) {
	meta := map[string]string{
		"Opener": fmt.Sprintf("%q", n.Opener),
		"Closer": fmt.Sprintf("%q", n.Closer),
	}
	gmast.DumpHelper(n, source, level, meta, nil)
}

// InlineMath represents inline LaTeX math delimited by $...$, $$...$$,
// \(...\), or \[...\].
type InlineMath struct {
	gmast.BaseInline

	Segment text.Segment
	Opener  string
	Closer  string
}

func NewInlineMath(segment text.Segment, opener, closer string) *InlineMath {
	return &InlineMath{
		Segment: segment,
		Opener:  opener,
		Closer:  closer,
	}
}

func (n *InlineMath) Kind() gmast.NodeKind {
	return KindInlineMath
}

func (n *InlineMath) Pos() int {
	return n.Segment.Start
}

func (n *InlineMath) Text(source []byte) []byte {
	return n.Segment.Value(source)
}

func (n *InlineMath) Dump(source []byte, level int) {
	meta := map[string]string{
		"Value":  fmt.Sprintf("%q", n.Text(source)),
		"Opener": fmt.Sprintf("%q", n.Opener),
		"Closer": fmt.Sprintf("%q", n.Closer),
	}
	gmast.DumpHelper(n, source, level, meta, nil)
}
