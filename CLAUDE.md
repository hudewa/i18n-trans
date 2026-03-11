# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 提供该代码仓库的工作指导。

## 项目概述

i18n-trans 是一个 Go 语言命令行工具，用于扫描代码文件中的中文文本，使用豆包 AI 翻译成多种语言，生成 i18n 数据库的 SQL 文件，并可选地将原始中文文本替换为 `module.identification` 格式。

## 构建和开发命令

```bash
# 构建二进制文件
go build -o i18n-trans ./cmd/gamoji-trans

# 运行测试
go test ./...

# 运行指定包的测试
go test ./internal/scanner/...
go test ./internal/translator/...

# 本地运行工具
go run ./cmd/gamoji-trans scan -d ./test
go run ./cmd/gamoji-trans process -d ./src -c ./config.yaml --replace
```

## 架构设计

代码采用分层架构，各层职责清晰：

### 入口点 (`cmd/gamoji-trans/main.go`)
- 使用 Cobra 框架管理 CLI 命令
- 命令：`scan`（扫描）、`translate`（翻译）、`process`（完整流程）、`init`（初始化配置）
- 配置优先级：命令行参数 > 配置文件 > 环境变量 (`DOUBAO_API_KEY`)

### 核心包

**`pkg/config/`** - 配置管理（使用 Viper）
- 支持 YAML 配置文件和环境变量
- 默认配置路径：`./config.yaml` 或 `~/.gamoji-trans/config`
- 关键配置：豆包 API 凭证、扫描模式、输出设置

**`internal/scanner/`** - 文件扫描和中文检测
- 基于扩展名过滤器递归扫描文件
- 使用正则表达式匹配引号字符串（`"..."` 和 `'...'`）
- 为每个唯一中文文本生成 MD5 哈希 ID
- 过滤图片路径（`.png`、`.jpg` 等）

**`internal/translator/`** - 豆包 API 集成
- 使用 `github.com/sashabaranov/go-openai` 客户端，自定义 base URL
- 并行批量翻译，每批 10 条以避免速率限制
- 目标语言：英语、印尼语、泰语、越南语、马来语
- 返回结构化结果，支持 JSON 解析降级

**`internal/generator/`** - SQL 文件生成
- 生成带时间戳的 SQL 文件（`i18n_YYYYMMDD_HHMMSS.sql`）
- 插入 `gamoji.i18n` 表，包含所有语言列
- 注意：SQL 输出中数据库名 `gamoji` 是硬编码的

**`internal/replacer/`** - 文件原地修改
- 将中文文本替换为 `module.id` 格式，保留原始引号
- 按行号/列号降序排序匹配项，避免索引偏移
- 支持 dry-run 模式预览变更

### 数据流

1. **扫描**：目录 → `[]scanner.Match`（文件路径、行号、列号、中文文本、ID）
2. **翻译**：唯一中文文本 → `[]translator.TranslationResult`（所有语言的翻译）
3. **生成**：翻译结果 → SQL INSERT 语句
4. **替换**（可选）：匹配项 → 文件修改（`module.id` 格式）

### 核心类型

```go
// scanner.Match - 发现的中文文本匹配项
type Match struct {
    FilePath    string // 完整文件路径
    Line        int    // 从 1 开始的行号
    Column      int    // 从 1 开始的列号
    RawText     string // 原始带引号文本
    QuoteType   string // " 或 '
    ChineseText string // 不带引号的文本
    ID          string // MD5 哈希（前 8 位）
}

// translator.TranslationResult - 翻译结果
type TranslationResult struct {
    Text  string // 原始中文
    ID    string // MD5 哈希
    Zh    string // 中文
    En    string // 英文
    Id    string // 印尼语（字段名：id_lang）
    Th    string // 泰语
    Vi    string // 越南语
    Ms    string // 马来语
    Error error
}
```

## 配置说明

默认使用腾讯云提供的 API Key：`sk-sp-BzqRlcJaaNzdOJbwgemHPg2EKaBH2jT75n5Q5OoUR3a1eEby`

用户也可以通过配置文件或环境变量自定义 API Key。

示例 `config.yaml`：

```yaml
doubao:
  api_key: "your-api-key"
  base_url: "https://ark.cn-beijing.volces.com/api/v3"
  model: "doubao-1.5-pro-32k"

scan:
  include_ext: [".go", ".js", ".vue", ".ts", ".tsx", ".jsx", ".json"]
  exclude_dirs: ["node_modules", ".git", "dist", "build", "vendor"]
  exclude_patterns: ["*_test.go", "*.min.js"]

output:
  sql_dir: "./sql"
  module_name: "doubao"
  updated_by: "doubao"
```

## 重要说明

- 代码中的二进制名是 `gamoji-trans`，但项目名是 `i18n-trans`
- 扫描器使用中文文本的 MD5 哈希生成 ID（取前 8 位）
- 翻译器每批处理 10 条文本以控制速率
- SQL 输出使用 `gamoji.i18n` 表名（硬编码）
- 替换器按反向顺序排序匹配项，以安全处理每文件的多次替换
