package parser

import (
	"regexp"
	"strings"
	"testing"
)

func TestExtractorProtectsFencedCodeAndMath(t *testing.T) {
	input := "# Title\n\n" +
		"Text with $E=mc^2$ and ordinary prose.\n\n" +
		"```go\n" +
		"func main() {\n" +
		"    fmt.Println(\"$not math$\")\n" +
		"}\n" +
		"```\n\n" +
		"$$\n" +
		"a^2 + b^2 = c^2\n" +
		"$$\n\n" +
		"After \\(\\alpha+\\beta\\).\n"

	result, err := Extract(input)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if got, want := len(result.Blocks), 4; got != want {
		t.Fatalf("protected block count = %d, want %d", got, want)
	}

	wantKinds := []ProtectedKind{
		ProtectedInlineMath,
		ProtectedFencedCode,
		ProtectedMathBlock,
		ProtectedInlineMath,
	}
	for i, want := range wantKinds {
		if result.Blocks[i].Kind != want {
			t.Fatalf("block %d kind = %s, want %s", i, result.Blocks[i].Kind, want)
		}
	}

	placeholderPattern := regexp.MustCompile(`\[\[BLOCK_UUID_[0-9a-f-]{36}\]\]`)
	if got := len(placeholderPattern.FindAllString(result.Markdown, -1)); got != 4 {
		t.Fatalf("placeholder count in markdown = %d, want 4\n%s", got, result.Markdown)
	}

	for _, forbidden := range []string{"$E=mc^2$", "fmt.Println", "a^2 + b^2", `\(\alpha+\beta\)`} {
		if strings.Contains(result.Markdown, forbidden) {
			t.Fatalf("clean markdown still contains protected content %q:\n%s", forbidden, result.Markdown)
		}
	}

	restored := result.Markdown
	for _, block := range result.Blocks {
		restored = strings.ReplaceAll(restored, block.Placeholder, block.Original)
	}
	if restored != input {
		t.Fatalf("restored markdown mismatch\nwant:\n%s\n\ngot:\n%s", input, restored)
	}
}

func TestExtractorLeavesCurrencyAlone(t *testing.T) {
	input := "The budget is $5 today, not a formula.\n"
	result, err := Extract(input)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(result.Blocks) != 0 {
		t.Fatalf("protected block count = %d, want 0", len(result.Blocks))
	}
	if result.Markdown != input {
		t.Fatalf("markdown changed unexpectedly: %q", result.Markdown)
	}
}
