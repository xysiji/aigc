package markdownutil

import "strings"

type ChunkKind string

const (
	ChunkText      ChunkKind = "text"
	ChunkSeparator ChunkKind = "separator"
)

type Chunk struct {
	Kind ChunkKind
	Text string
}

type lineSpan struct {
	start   int
	bodyEnd int
	blank   bool
}

// SplitChunks breaks markdown into rewriteable text chunks and exact separator
// chunks. Separator chunks preserve original line endings and blank-line runs so
// the document layout can be reconstructed byte-for-byte after rewriting.
func SplitChunks(markdown string) []Chunk {
	if markdown == "" {
		return nil
	}

	lines := splitLines(markdown)
	chunks := make([]Chunk, 0, len(lines))

	for i := 0; i < len(lines); {
		if lines[i].blank {
			j := i + 1
			for j < len(lines) && lines[j].blank {
				j++
			}

			end := len(markdown)
			if j < len(lines) {
				end = lines[j].start
			}
			chunks = append(chunks, Chunk{
				Kind: ChunkSeparator,
				Text: markdown[lines[i].start:end],
			})
			i = j
			continue
		}

		j := i
		for j+1 < len(lines) && !lines[j+1].blank {
			j++
		}

		textEnd := lines[j].bodyEnd
		chunks = append(chunks, Chunk{
			Kind: ChunkText,
			Text: markdown[lines[i].start:textEnd],
		})

		if textEnd == len(markdown) {
			break
		}

		if j+1 >= len(lines) {
			chunks = append(chunks, Chunk{
				Kind: ChunkSeparator,
				Text: markdown[textEnd:],
			})
			break
		}

		if lines[j+1].blank {
			k := j + 1
			for k < len(lines) && lines[k].blank {
				k++
			}

			end := len(markdown)
			if k < len(lines) {
				end = lines[k].start
			}
			chunks = append(chunks, Chunk{
				Kind: ChunkSeparator,
				Text: markdown[textEnd:end],
			})
			i = k
			continue
		}

		i = j + 1
	}

	return chunks
}

func JoinChunks(chunks []Chunk) string {
	if len(chunks) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, chunk := range chunks {
		builder.WriteString(chunk.Text)
	}
	return builder.String()
}

func splitLines(markdown string) []lineSpan {
	lines := make([]lineSpan, 0, strings.Count(markdown, "\n")+1)

	for start := 0; start < len(markdown); {
		end := start
		for end < len(markdown) && markdown[end] != '\n' && markdown[end] != '\r' {
			end++
		}

		bodyEnd := end
		if end < len(markdown) {
			if markdown[end] == '\r' {
				end++
				if end < len(markdown) && markdown[end] == '\n' {
					end++
				}
			} else {
				end++
			}
		}

		lines = append(lines, lineSpan{
			start:   start,
			bodyEnd: bodyEnd,
			blank:   isBlankLine(markdown[start:bodyEnd]),
		})
		start = end
	}

	return lines
}

func isBlankLine(line string) bool {
	for _, r := range line {
		if r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}
