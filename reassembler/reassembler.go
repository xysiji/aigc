package reassembler

import (
	"log"
	"regexp"
	"strings"

	"github.com/example/docupolisher/parser"
	"github.com/example/docupolisher/pkg/markdownutil"
)

var placeholderPattern = regexp.MustCompile(`\[\[BLOCK_UUID_[0-9a-f-]{36}\]\]`)

// Reassemble 接收原始解析结果和重写后的文本，执行安全校验、段落级回退与高保真重组
func Reassemble(originalMarkdown string, rewrittenMarkdown string, extraction *parser.ExtractionResult) string {
	if extraction == nil {
		return rewrittenMarkdown
	}

	originalChunks := markdownutil.SplitChunks(originalMarkdown)
	rewrittenChunks := markdownutil.SplitChunks(rewrittenMarkdown)

	// 2. 校验与 Fallback 机制
	// 只有在分块结构一致的情况下才能进行精准的段落级回退
	if sameChunkLayout(originalChunks, rewrittenChunks) {
		for i, originalChunk := range originalChunks {
			if originalChunk.Kind != markdownutil.ChunkText {
				continue
			}
			for placeholder := range extraction.Protected {
				if strings.Count(originalChunk.Text, placeholder) != strings.Count(rewrittenChunks[i].Text, placeholder) {
					log.Printf("[WARNING] 检测到大模型吞噬或复制了占位符！第 %d 个文本块中的 %s 数量不一致，触发文本块回退。", i+1, placeholder)
					rewrittenChunks[i] = originalChunk
					break
				}
			}
		}
		// 将可能经过 Fallback 修复的文本重新组合
		rewrittenMarkdown = markdownutil.JoinChunks(rewrittenChunks)
	} else {
		log.Printf("[WARNING] 重写前后分块结构不一致 (原:%d, 新:%d)，跳过文本块级校验，执行全局完整性校验。", len(originalChunks), len(rewrittenChunks))
	}

	if !hasExpectedPlaceholderCounts(originalMarkdown, rewrittenMarkdown, extraction.Protected) {
		log.Printf("[WARNING] 全局占位符完整性校验失败，已回退整篇文档。")
		rewrittenMarkdown = originalMarkdown
	}
	if hasUnknownPlaceholders(rewrittenMarkdown, extraction.Protected) {
		log.Printf("[WARNING] 检测到未知占位符，已回退整篇文档。")
		rewrittenMarkdown = originalMarkdown
	}

	// 3. 高保真重组：执行极其精准的字符串替换
	// 将占位符替换回真实的原始 Block（代码块、公式等）
	finalText := rewrittenMarkdown
	for placeholder, originalContent := range extraction.Protected {
		// 使用 ReplaceAll 确保同一个占位符如果意外重复出现也能被全部替换回原文
		finalText = strings.ReplaceAll(finalText, placeholder, originalContent)
	}

	return finalText
}

func sameChunkLayout(left, right []markdownutil.Chunk) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Kind != right[i].Kind {
			return false
		}
	}
	return true
}

func hasExpectedPlaceholderCounts(original, rewritten string, protected map[string]string) bool {
	for placeholder := range protected {
		if strings.Count(original, placeholder) != strings.Count(rewritten, placeholder) {
			return false
		}
	}
	return true
}

func hasUnknownPlaceholders(markdown string, protected map[string]string) bool {
	matches := placeholderPattern.FindAllString(markdown, -1)
	for _, match := range matches {
		if _, ok := protected[match]; !ok {
			return true
		}
	}
	return false
}
