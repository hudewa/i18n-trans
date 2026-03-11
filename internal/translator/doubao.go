package translator

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
)

// Translator 豆包API翻译器，用于将中文文本翻译成多种语言
// 支持并发批量翻译，通过豆包（字节跳动）的大模型API实现
type Translator struct {
	client *openai.Client // OpenAI 格式的 HTTP 客户端
	model  string         // 豆包模型名称，如 "doubao-1.5-pro-32k"
}

// TranslationResult 单条文本的翻译结果，包含所有目标语言的翻译
type TranslationResult struct {
	Text   string `json:"text"`    // 原始中文文本
	ID     string `json:"id"`      // 文本的 MD5 哈希 ID（前8位）
	Zh     string `json:"zh"`      // 中文（原文）
	En     string `json:"en"`      // 英文翻译
	Id     string `json:"id_lang"` // 印尼语翻译（字段名 id_lang 是因为 id 是 Go 关键字）
	Th     string `json:"th"`      // 泰语翻译
	Vi     string `json:"vi"`      // 越南语翻译
	Ms     string `json:"ms"`      // 马来语翻译
	Error  error  `json:"-"`       // 翻译过程中的错误（不序列化到 JSON）
}

// New 创建一个新的翻译器实例
//
// 参数说明:
//   - apiKey: 豆包 API 的访问密钥，用于身份验证
//             示例: "sk-sp-BzqRlcJaaNzdOJbwgemHPg2EKaBH2jT75n5Q5OoUR3a1eEby"
//   - baseURL: 豆包 API 的基础 URL 地址
//              示例: "https://ark.cn-beijing.volces.com/api/v3"
//   - model: 使用的豆包模型名称
//            示例: "doubao-1.5-pro-32k" (32k 上下文窗口版本)
//
// 返回值:
//   - *Translator: 配置好的翻译器实例
//
// 使用示例:
//
//	trans := translator.New(
//	    "sk-xxxxx",
//	    "https://ark.cn-beijing.volces.com/api/v3",
//	    "doubao-1.5-pro-32k",
//	)
func New(apiKey, baseURL, model string) *Translator {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL
	client := openai.NewClientWithConfig(config)

	return &Translator{
		client: client,
		model:  model,
	}
}

// TranslateTexts 批量翻译多个文本（并发执行）
//
// 参数说明:
//   - ctx: 上下文，用于取消操作和设置超时
//   - texts: 需要翻译的中文文本数组（必填）
//
// 返回值:
//   - []TranslationResult: 翻译结果数组，顺序与输入 texts 一致
//     每个结果包含自动生成的 MD5 ID
//   - error: 翻译过程中的错误（如果有）
//
// 特性:
//   - ID 自动生成：根据文本内容计算 MD5 哈希（前8位）
//   - 自动分批处理，每批 10 条文本，避免 API 速率限制
//   - 并发执行多个批次，提高翻译效率
//   - 线程安全，使用互斥锁保护结果收集
//
// 使用示例:
//
//	texts := []string{"你好", "世界", "欢迎"}
//	results, err := trans.TranslateTexts(ctx, texts)
//	for _, r := range results {
//	    fmt.Printf("ID: %s, EN: %s\n", r.ID, r.En)
//	}
func (t *Translator) TranslateTexts(ctx context.Context, texts []string) ([]TranslationResult, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 自动生成 IDs（基于文本内容的 MD5 前8位）
	ids := generateIDs(texts)

	// Process in batches to avoid rate limits
	const batchSize = 10
	var results []TranslationResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batchTexts := texts[i:end]
		batchIDs := ids[i:end]

		wg.Add(1)
		go func(textsBatch, idsBatch []string) {
			defer wg.Done()

			batchResults, err := t.translateBatch(ctx, textsBatch, idsBatch)
			if err != nil {
				errChan <- fmt.Errorf("error translating batch: %w", err)
				return
			}

			mu.Lock()
			results = append(results, batchResults...)
			mu.Unlock()
		}(batchTexts, batchIDs)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return results, fmt.Errorf("translation errors: %s", strings.Join(errs, "; "))
	}

	return results, nil
}

// translateBatch 翻译一批文本（内部方法）
//
// 参数说明:
//   - ctx: 上下文
//   - texts: 该批次的中文文本数组（最多 10 条）
//   - ids: 对应的 ID 数组
//
// 返回值:
//   - []TranslationResult: 翻译结果
//   - error: API 调用或解析错误
//
// 实现细节:
//   - 构建提示词，要求 AI 返回特定格式的 JSON
//   - 调用豆包 API 的 Chat Completion 接口
//   - 解析 JSON 响应，支持多种格式容错
//   - 自动填充 ID 和原始文本到结果中
func (t *Translator) translateBatch(ctx context.Context, texts []string, ids []string) ([]TranslationResult, error) {
	// Build prompt for batch translation
	prompt := buildBatchPrompt(texts)

	resp, err := t.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: t.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a professional translator. Translate the given Chinese texts to multiple languages. Return results in strict JSON format.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: 0.3,
	})

	if err != nil {
		return nil, fmt.Errorf("API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from API")
	}

	// Parse the response
	content := resp.Choices[0].Message.Content

	// Try to extract JSON from markdown code blocks if present
	content = extractJSON(content)

	var translations []TranslationResult
	if err := json.Unmarshal([]byte(content), &translations); err != nil {
		// Try alternative format - array of objects
		var altFormat []map[string]string
		if err2 := json.Unmarshal([]byte(content), &altFormat); err2 == nil {
			for i, item := range altFormat {
				if i < len(texts) {
					translations = append(translations, TranslationResult{
						Text: texts[i],
						ID:   ids[i],
						Zh:   item["zh"],
						En:   item["en"],
						Id:   item["id"],
						Th:   item["th"],
						Vi:   item["vi"],
						Ms:   item["ms"],
					})
				}
			}
		} else {
			return nil, fmt.Errorf("failed to parse response: %w, content: %s", err, content)
		}
	}

	// Ensure IDs are set
	for i := range translations {
		if i < len(ids) {
			translations[i].ID = ids[i]
			translations[i].Text = texts[i]
		}
	}

	return translations, nil
}

// buildBatchPrompt 构建批量翻译的提示词
//
// 参数说明:
//   - texts: 需要翻译的中文文本数组
//
// 返回值:
//   - string: 构建好的提示词，包含格式要求和待翻译文本
//
// 提示词说明:
//   - 要求 AI 将文本翻译成 5 种语言：英、印尼、泰、越南、马来
//   - 要求返回严格的 JSON 数组格式
//   - 每个对象包含字段: zh, en, id, th, vi, ms
func buildBatchPrompt(texts []string) string {
	var sb strings.Builder
	sb.WriteString("Translate the following Chinese texts to multiple languages. ")
	sb.WriteString("Return a JSON array where each object has the following structure:\n")
	sb.WriteString(`{"zh": "original chinese", "en": "english", "id": "indonesian", "th": "thai", "vi": "vietnamese", "ms": "malay"}`)
	sb.WriteString("\n\nTexts to translate:\n")

	for i, text := range texts {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, text))
	}

	sb.WriteString("\nRespond with ONLY the JSON array, no other text.")
	return sb.String()
}

// extractJSON 从 Markdown 代码块中提取 JSON 内容
//
// 参数说明:
//   - content: API 返回的原始文本内容
//
// 返回值:
//   - string: 提取后的纯 JSON 字符串
//
// 支持格式:
//   - ```json\n{...}\n```  - 带 json 标记的代码块
//   - ```\n{...}\n```      - 无标记代码块
//   - {...}               - 纯 JSON（直接返回）
//   - 自动去除前后空白字符
func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	// Check for markdown code blocks
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		return strings.TrimSpace(content)
	}

	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		return strings.TrimSpace(content)
	}

	return content
}

// TranslateSingle 翻译单个文本（便捷方法）
//
// 参数说明:
//   - ctx: 上下文
//   - text: 需要翻译的中文文本
//
// 返回值:
//   - *TranslationResult: 翻译结果
//   - error: 翻译错误（如果有）
//
// 说明:
//   - 内部调用 TranslateTexts 方法
//   - 适用于只需要翻译一条文本的场景
//
// 使用示例:
//
//	result, err := trans.TranslateSingle(ctx, "你好世界")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.En) // 输出: Hello World
func (t *Translator) TranslateSingle(ctx context.Context, text string) (*TranslationResult, error) {
	results, err := t.TranslateTexts(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no translation result")
	}
	return &results[0], nil
}

// generateIDs 为文本数组生成 MD5 哈希 ID
//
// 参数说明:
//   - texts: 中文文本数组
//
// 返回值:
//   - []string: 每个文本对应的 MD5 哈希（前8位）
//
// 说明:
//   - 使用 MD5 算法计算文本哈希
//   - 取前8位作为短 ID，足够唯一且便于阅读
//   - 相同文本总是生成相同的 ID
func generateIDs(texts []string) []string {
	ids := make([]string, len(texts))
	for i, text := range texts {
		hash := md5.Sum([]byte(text))
		ids[i] = hex.EncodeToString(hash[:])[:8]
	}
	return ids
}
