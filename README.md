# DocuPolisher

DocuPolisher is a Go CLI that rewrites Markdown prose with an LLM while
preserving protected regions such as fenced code blocks and LaTeX math.

## What it does

1. Parses Markdown and replaces protected regions with UUID placeholders.
2. Sends only rewriteable text chunks to the model.
3. Validates placeholder integrity after rewriting.
4. Restores the original protected content into the final document.

The current implementation explicitly protects:

- fenced code blocks
- display math blocks delimited by `$$...$$` or `\[...\]`
- inline math delimited by `$...$`, `$$...$$`, `\(...\)`, or `\[...\]`

## Usage

```bash
go run ./cmd/docupolisher \
  -in input.md \
  -out polished_output.md \
  -api-key "$OPENAI_API_KEY" \
  -model gpt-4-turbo
```

Optional flags:

- `-base-url` to point at a compatible OpenAI-style endpoint
- `-out` to change the output path
- `-model` to choose a different model

## Safety behavior

- Original separator layout is preserved across `LF` and `CRLF` documents.
- If a rewritten text chunk drops or duplicates a protected placeholder, that
  chunk falls back to the original source.
- If global placeholder integrity still fails, the whole document falls back to
  the original extracted markdown before protected content is restored.

## Development

```bash
go test ./...
go build ./cmd/docupolisher
```
