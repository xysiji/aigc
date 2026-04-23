package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/example/docupolisher/parser"
	"github.com/example/docupolisher/reassembler"
	"github.com/example/docupolisher/rewriter"
)

func main() {
	// 1. 定义命令行参数
	inputFile := flag.String("in", "", "输入的 Markdown 源文件路径 (必填)")
	outputFile := flag.String("out", "polished_output.md", "输出的 Markdown 文件路径")
	apiKey := flag.String("api-key", "", "OpenAI/DeepSeek API Key (必填)")
	baseURL := flag.String("base-url", "https://api.openai.com/v1", "API 的 Base URL (可选，如需切换 DeepSeek 请修改)")
	model := flag.String("model", "gpt-4-turbo", "使用的大模型名称 (可选)")

	flag.Parse()

	// 简单的参数校验
	if *inputFile == "" || *apiKey == "" {
		log.Println("用法错误: 请提供必要的参数。")
		flag.Usage()
		os.Exit(1)
	}

	log.Printf("==> 启动 DocuPolisher \n==> 目标文件: %s", *inputFile)

	// 2. 读取原始 Markdown 文件
	sourceBytes, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("读取输入文件失败: %v", err)
	}
	originalMarkdown := string(sourceBytes)

	// ==========================================
	// [第一层] 解析与安全隔离层 (Parser Layer)
	// ==========================================
	log.Println("[1/3] 正在执行解析与底层安全隔离...")
	extractionResult, err := parser.Extract(originalMarkdown)
	if err != nil {
		log.Fatalf("文档解析提取失败: %v", err)
	}
	log.Printf("      成功隔离 %d 个代码块/数学公式节点。", len(extractionResult.Blocks))

	// ==========================================
	// [第二层] 并发改写与降维层 (Rewriter Layer)
	// ==========================================
	log.Println("[2/3] 正在调用 LLM 进行并发去机器味重写...")
	rw := rewriter.NewRewriter(rewriter.Config{
		AuthToken:   *apiKey,
		BaseURL:     *baseURL,
		Model:       *model,
		Concurrency: 5, // 控制并发数，保护 API 速率
	})

	ctx := context.Background()
	// 将包含 UUID 占位符的纯净文本送入大模型
	rewrittenMarkdown, err := rw.ProcessDocument(ctx, extractionResult.Markdown)
	if err != nil {
		log.Fatalf("并发重写过程发生致命错误: %v", err)
	}

	// ==========================================
	// [第三层] 高保真重组与输出层 (Reassembler Layer)
	// ==========================================
	log.Println("[3/3] 正在执行校验与高保真碎片重组...")
	finalMarkdown := reassembler.Reassemble(extractionResult.Markdown, rewrittenMarkdown, extractionResult)

	// 4. 安全输出纯净文件
	err = os.WriteFile(*outputFile, []byte(finalMarkdown), 0644)
	if err != nil {
		log.Fatalf("写入输出文件失败: %v", err)
	}

	log.Printf("==> 任务完成！清洗与重写后的纯净文档已保存至: %s", *outputFile)
}
