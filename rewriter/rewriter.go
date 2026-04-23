package rewriter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"

	"github.com/example/docupolisher/pkg/markdownutil"
)

// 核心常量定义：严格按照架构要求的 System Prompt
const RewritePrompt = `你是一位资深架构师。请重写以下段落以提升原创感。
要求：
1. 大幅提高 Burstiness（突发性）：必须交替使用极短句（5字以内）和复杂的长复合句，打破句式平衡。
2. 提高 Perplexity（困惑度）：绝对禁止使用‘综上所述’、‘显而易见’、‘不仅如此’等常见过渡模板词，使用更具主观色彩的工程师口语书面化表达。
3. 严禁修改、翻译或吞噬任何 [[BLOCK_UUID_xxx]] 占位符。
4. 保持核心技术术语和逻辑不变。`

// Config 包含 Rewriter 所需的配置项
type Config struct {
	AuthToken   string
	BaseURL     string // 用于兼容 DeepSeek 等其他 API
	Model       string // 模型名称，如 "deepseek-chat" 或 "gpt-4-turbo"
	Temperature float32
	Concurrency int           // 最大并发数
	MaxRetries  int           // 最大重试次数
	Timeout     time.Duration // 单次 API 调用的超时时间
}

// Rewriter 负责处理并发重写逻辑
type Rewriter struct {
	client      *openai.Client
	config      Config
	rewriteText func(context.Context, string) (string, error)
}

// 内部结构体：用于在 Channel 中传递作业和结果，以保证最终段落顺序不变
type rewriteJob struct {
	index int
	text  string
}

type rewriteResult struct {
	index int
	text  string
	err   error
}

// NewRewriter 初始化一个新的重写器
func NewRewriter(cfg Config) *Rewriter {
	// 设置默认值
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 5 // 默认 5 并发
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.85 // 推荐配置
	}

	// 初始化 OpenAI 客户端配置
	clientConfig := openai.DefaultConfig(cfg.AuthToken)
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}

	rw := &Rewriter{
		client: openai.NewClientWithConfig(clientConfig),
		config: cfg,
	}
	rw.rewriteText = rw.callAPIWithRetry
	return rw
}

// ProcessDocument 接收包含占位符的 Markdown 文本，拆分、并发重写后重新拼接
func (r *Rewriter) ProcessDocument(ctx context.Context, markdown string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	chunks := markdownutil.SplitChunks(markdown)
	if len(chunks) == 0 {
		return "", nil
	}

	rewrittenChunks := make([]markdownutil.Chunk, len(chunks))
	jobCount := 0
	for i, chunk := range chunks {
		if chunk.Kind != markdownutil.ChunkText || strings.TrimSpace(chunk.Text) == "" {
			rewrittenChunks[i] = chunk
			continue
		}
		jobCount++
	}

	if jobCount == 0 {
		return markdownutil.JoinChunks(rewrittenChunks), nil
	}

	jobs := make(chan rewriteJob, jobCount)
	results := make(chan rewriteResult, jobCount)
	var wg sync.WaitGroup

	// 1. 启动 Worker 线程池
	for i := 0; i < r.config.Concurrency; i++ {
		wg.Add(1)
		go r.worker(ctx, &wg, jobs, results)
	}

	// 2. 分发任务
	for i, chunk := range chunks {
		if chunk.Kind != markdownutil.ChunkText || strings.TrimSpace(chunk.Text) == "" {
			continue
		}
		jobs <- rewriteJob{index: i, text: chunk.Text}
	}
	close(jobs)

	// 3. 等待所有 Worker 完成后关闭结果通道
	go func() {
		wg.Wait()
		close(results)
	}()

	// 4. 收集结果并保证顺序
	var canceled bool
	for res := range results {
		if res.err != nil {
			if errors.Is(res.err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				canceled = true
				rewrittenChunks[res.index] = chunks[res.index]
				continue
			}

			// 如果重试后依然失败，记录警告并 Fallback 保留原文
			log.Printf("[WARNING] 第 %d 段重写失败，已回退为原文。原因: %v", res.index, res.err)
			rewrittenChunks[res.index] = chunks[res.index]
		} else {
			rewrittenChunks[res.index] = markdownutil.Chunk{
				Kind: markdownutil.ChunkText,
				Text: res.text,
			}
		}
	}

	if canceled {
		return "", context.Canceled
	}

	// 重新组合文档
	return markdownutil.JoinChunks(rewrittenChunks), nil
}

// worker 从 jobs 通道读取段落，调用 API 进行处理
func (r *Rewriter) worker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan rewriteJob, results chan<- rewriteResult) {
	defer wg.Done()

	for job := range jobs {
		// 跳过空段落或纯空格段落
		if strings.TrimSpace(job.text) == "" {
			results <- rewriteResult{index: job.index, text: job.text, err: nil}
			continue
		}

		rewrittenText, err := r.rewriteText(ctx, job.text)
		results <- rewriteResult{index: job.index, text: rewrittenText, err: err}
	}
}

// callAPIWithRetry 封装了重试和超时逻辑的单次 API 调用
func (r *Rewriter) callAPIWithRetry(ctx context.Context, text string) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		// 为每次请求赋予独立的超时 Context
		reqCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)

		resp, err := r.client.CreateChatCompletion(
			reqCtx,
			openai.ChatCompletionRequest{
				Model:       r.config.Model,
				Temperature: r.config.Temperature,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: RewritePrompt,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: text,
					},
				},
			},
		)

		cancel() // 释放 context 资源

		if err == nil && len(resp.Choices) > 0 {
			// 成功获取结果
			return cleanLLMOutput(resp.Choices[0].Message.Content), nil
		}

		lastErr = err
		// 如果上下文被外部取消，则直接退出，不重试
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		// 指数退避策略（Exponential Backoff）- 避免重试风暴
		if err := sleepWithContext(ctx, time.Duration(attempt)*time.Second); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("达到最大重试次数: %w", lastErr)
}

// cleanLLMOutput removes a single outer fenced block wrapper that some models
// add around otherwise plain-text responses. It only unwraps when the whole
// response is exactly one fenced block, and it preserves the inner content
// byte-for-byte apart from the fence-adjacent line breaks.
func cleanLLMOutput(text string) string {
	fenceLen, contentStart, ok := parseOpeningFence(text)
	if !ok {
		return text
	}

	closeStart, ok := parseClosingFence(text, fenceLen)
	if !ok || closeStart < contentStart {
		return text
	}

	contentEnd := trimTrailingFenceNewline(text, closeStart)
	if contentEnd < contentStart {
		contentEnd = contentStart
	}
	return text[contentStart:contentEnd]
}

func parseOpeningFence(text string) (int, int, bool) {
	fenceLen := 0
	for fenceLen < len(text) && text[fenceLen] == '`' {
		fenceLen++
	}
	if fenceLen < 3 {
		return 0, 0, false
	}

	lineEnd := 0
	for lineEnd < len(text) && text[lineEnd] != '\n' && text[lineEnd] != '\r' {
		lineEnd++
	}
	if lineEnd == len(text) {
		return 0, 0, false
	}

	return fenceLen, advancePastLineBreak(text, lineEnd), true
}

func parseClosingFence(text string, minFenceLen int) (int, bool) {
	end := trimSingleTrailingLineBreak(text, len(text))
	lineStart := lastLineStart(text, end)
	if lineStart >= end {
		return 0, false
	}

	fenceLen := 0
	for lineStart+fenceLen < end && text[lineStart+fenceLen] == '`' {
		fenceLen++
	}
	if fenceLen < minFenceLen {
		return 0, false
	}

	for i := lineStart + fenceLen; i < end; i++ {
		if text[i] != ' ' && text[i] != '\t' {
			return 0, false
		}
	}

	return lineStart, true
}

func advancePastLineBreak(text string, pos int) int {
	if pos >= len(text) {
		return len(text)
	}
	if text[pos] == '\r' {
		pos++
		if pos < len(text) && text[pos] == '\n' {
			pos++
		}
		return pos
	}
	if text[pos] == '\n' {
		return pos + 1
	}
	return pos
}

func trimSingleTrailingLineBreak(text string, end int) int {
	if end >= 2 && text[end-2] == '\r' && text[end-1] == '\n' {
		return end - 2
	}
	if end >= 1 && (text[end-1] == '\n' || text[end-1] == '\r') {
		return end - 1
	}
	return end
}

func trimTrailingFenceNewline(text string, end int) int {
	return trimSingleTrailingLineBreak(text, end)
}

func lastLineStart(text string, end int) int {
	for i := end - 1; i >= 0; i-- {
		if text[i] == '\n' || text[i] == '\r' {
			return i + 1
		}
	}
	return 0
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
