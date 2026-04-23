package parser

import (
	"bytes"

	"github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	gmparser "github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// MathExtension registers DocuPolisher's lightweight LaTeX parsers with
// goldmark. goldmark does not ship a core math grammar, so we keep the parser
// small and focused on preservation instead of rendering.
type MathExtension struct{}

func (MathExtension) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(
		gmparser.WithBlockParsers(util.Prioritized(NewMathBlockParser(), 650)),
		gmparser.WithInlineParsers(util.Prioritized(NewInlineMathParser(), 80)),
	)
}

type mathBlockParser struct{}

var mathBlockContextKey = gmparser.NewContextKey()

func NewMathBlockParser() gmparser.BlockParser {
	return &mathBlockParser{}
}

func (p *mathBlockParser) Trigger() []byte {
	return []byte{'$', '\\'}
}

func (p *mathBlockParser) Open(parent gmast.Node, reader text.Reader, pc gmparser.Context) (gmast.Node, gmparser.State) {
	line, segment := reader.PeekLine()
	if len(line) == 0 || pc.BlockOffset() < 0 || pc.BlockOffset() >= len(line) {
		return nil, gmparser.NoChildren
	}

	pos := pc.BlockOffset()
	rest := line[pos:]
	opener, closer, ok := mathBlockFence(rest)
	if !ok {
		return nil, gmparser.NoChildren
	}

	start := absoluteOffset(segment, pos)
	node := NewMathBlock(opener, closer, start)
	pc.Set(mathBlockContextKey, node)
	return node, gmparser.NoChildren
}

func (p *mathBlockParser) Continue(node gmast.Node, reader text.Reader, pc gmparser.Context) gmparser.State {
	mathNode, ok := node.(*MathBlock)
	if !ok {
		return gmparser.Close
	}

	line, segment := reader.PeekLine()
	if line == nil {
		mathNode.StopOffset = len(reader.Source())
		return gmparser.Close
	}

	width, pos := util.IndentWidth(line, reader.LineOffset())
	if width < 4 && pos < len(line) && bytes.HasPrefix(line[pos:], []byte(mathNode.Closer)) {
		after := line[pos+len(mathNode.Closer):]
		if util.IsBlank(after) {
			mathNode.StopOffset = lineContentEnd(reader.Source(), segment.Start)
			reader.AdvanceToEOL()
			return gmparser.Close
		}
	}

	content := text.NewSegment(segment.Start, lineContentEnd(reader.Source(), segment.Start))
	content.ForceNewline = true
	mathNode.Lines().Append(content)
	reader.AdvanceToEOL()
	return gmparser.Continue | gmparser.NoChildren
}

func (p *mathBlockParser) Close(node gmast.Node, reader text.Reader, pc gmparser.Context) {
	if mathNode, ok := node.(*MathBlock); ok && mathNode.StopOffset < 0 {
		mathNode.StopOffset = len(reader.Source())
	}
	if pc.Get(mathBlockContextKey) == node {
		pc.Set(mathBlockContextKey, nil)
	}
}

func (p *mathBlockParser) CanInterruptParagraph() bool {
	return true
}

func (p *mathBlockParser) CanAcceptIndentedLine() bool {
	return false
}

type inlineMathParser struct{}

func NewInlineMathParser() gmparser.InlineParser {
	return &inlineMathParser{}
}

func (p *inlineMathParser) Trigger() []byte {
	return []byte{'$', '\\'}
}

func (p *inlineMathParser) Parse(parent gmast.Node, reader text.Reader, pc gmparser.Context) gmast.Node {
	line, segment := reader.PeekLine()
	if len(line) == 0 {
		return nil
	}

	switch line[0] {
	case '$':
		return p.parseDollarMath(line, segment, reader)
	case '\\':
		return p.parseBracketMath(line, segment, reader)
	default:
		return nil
	}
}

func (p *inlineMathParser) parseDollarMath(line []byte, segment text.Segment, reader text.Reader) gmast.Node {
	opener := "$"
	closer := "$"
	delimiterLen := 1
	if len(line) >= 2 && line[1] == '$' {
		opener = "$$"
		closer = "$$"
		delimiterLen = 2
	}

	if len(line) <= delimiterLen {
		return nil
	}
	if delimiterLen == 1 && isSpaceOrNewline(line[delimiterLen]) {
		return nil
	}

	closeAt := findUnescaped(line, []byte(closer), delimiterLen)
	if closeAt < 0 {
		return nil
	}
	if delimiterLen == 1 && closeAt > 0 && isSpaceOrNewline(line[closeAt-1]) {
		return nil
	}

	stop := closeAt + delimiterLen
	reader.Advance(stop)
	return NewInlineMath(text.NewSegment(segment.Start, segment.Start+stop), opener, closer)
}

func (p *inlineMathParser) parseBracketMath(line []byte, segment text.Segment, reader text.Reader) gmast.Node {
	if len(line) < 4 || line[0] != '\\' {
		return nil
	}

	var opener, closer string
	switch line[1] {
	case '(':
		opener, closer = `\(`, `\)`
	case '[':
		opener, closer = `\[`, `\]`
	default:
		return nil
	}

	closeAt := bytes.Index(line[2:], []byte(closer))
	if closeAt < 0 {
		return nil
	}

	stop := closeAt + 2 + len(closer)
	reader.Advance(stop)
	return NewInlineMath(text.NewSegment(segment.Start, segment.Start+stop), opener, closer)
}

func mathBlockFence(rest []byte) (string, string, bool) {
	if bytes.HasPrefix(rest, []byte("$$")) && !bytes.HasPrefix(rest, []byte("$$$")) {
		if util.IsBlank(rest[2:]) {
			return "$$", "$$", true
		}
		return "", "", false
	}
	if bytes.HasPrefix(rest, []byte(`\[`)) {
		if util.IsBlank(rest[2:]) {
			return `\[`, `\]`, true
		}
	}
	return "", "", false
}

func findUnescaped(line []byte, needle []byte, start int) int {
	for i := start; i+len(needle) <= len(line); i++ {
		if !bytes.HasPrefix(line[i:], needle) {
			continue
		}
		if isEscaped(line, i) {
			continue
		}
		return i
	}
	return -1
}

func isEscaped(line []byte, pos int) bool {
	backslashes := 0
	for i := pos - 1; i >= 0 && line[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func isSpaceOrNewline(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
