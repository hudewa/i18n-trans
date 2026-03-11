# i18n-trans

一个 Go 语言命令行工具，用于扫描、翻译和替换代码文件中的中文文本。

## 功能特性

- 🔍 扫描代码文件中的中文文本（支持单引号和双引号包裹）
- 🌐 使用豆包 AI 将中文翻译成多种语言
- 📝 生成 i18n 数据库的 SQL 插入文件
- 🔄 将原始中文文本替换为 `module.identification` 格式

## 安装

```bash
go install github.com/hudewa/i18n-trans/cmd/i18n-trans@latest
```

或者本地克隆构建：

```bash
git clone https://github.com/hudewa/i18n-trans.git
cd i18n-trans
go build -o i18n-trans ./cmd/i18n-trans
```

## 快速开始

1. **创建配置文件：**

```bash
i18n-trans init -o config.yaml
```

2. **编辑配置文件**，填入你的豆包 API 密钥。

3. **执行完整流程：**

```bash
i18n-trans process -d ./src -c ./config.yaml --replace
```

## 命令说明

### `scan` - 扫描

扫描目录中的中文文本（不翻译）：

```bash
i18n-trans scan -d ./src
```

### `translate` - 翻译

扫描并翻译中文文本，生成 SQL 文件：

```bash
i18n-trans translate -d ./src -o ./sql -k YOUR_API_KEY
```

### `process` - 完整流程

执行完整工作流：扫描、翻译、生成 SQL、可选替换：

```bash
# 预览替换效果（不实际修改）
i18n-trans process -d ./src --dry-run

# 完整流程并执行替换
i18n-trans process -d ./src -o ./sql --replace

# 使用配置文件
i18n-trans process -c ./config.yaml --replace
```

### `init` - 初始化配置

创建示例配置文件：

```bash
i18n-trans init -o config.yaml
```

## 配置说明

配置优先级（从高到低）：

1. **命令行参数**
2. **配置文件**（通过 `-c` 指定或默认位置）
3. **环境变量**（如 `DOUBAO_API_KEY`）

### 配置文件示例

```yaml
# 豆包 API 配置
doubao:
  api_key: "your-api-key-here"
  base_url: "https://ark.cn-beijing.volces.com/api/v3"
  model: "doubao-1.5-pro-32k"

# 扫描器配置
scan:
  include_ext:
    - ".go"
    - ".js"
    - ".vue"
    - ".ts"
    - ".tsx"
    - ".jsx"
    - ".json"
  exclude_dirs:
    - "node_modules"
    - ".git"
    - "dist"
    - "build"
  exclude_patterns:
    - "*_test.go"
    - "*.min.js"

# 输出配置
output:
  sql_dir: "./sql"
  module_name: "doubao"
  updated_by: "doubao"
```

## 支持的语言

工具将中文翻译成以下语言：

- `en` - 英语
- `id` - 印尼语
- `th` - 泰语
- `vi` - 越南语
- `ms` - 马来语

## SQL 输出格式

生成的 SQL 文件格式如下：

```sql
INSERT INTO i18n (type, module, identification, zh_lan, en_lan, id_lan, th_lan, vi_lan, ms_lan, updated_by)
VALUES (1, 'doubao', 'a1b2c3d4', '你好世界', 'Hello World', 'Halo Dunia', 'สวัสดีชาวโลก', 'Xin chào thế giới', 'Hai Dunia', 'doubao');
```

## 工作原理

1. **扫描**：递归扫描指定目录中的文件
2. **中文检测**：使用正则表达式查找单双引号中的中文文本
3. **过滤**：自动跳过图片文件路径（`.png`, `.webp`, `.jpg` 等）
4. **翻译**：将唯一的中文文本批量发送到豆包 AI 进行翻译
5. **SQL 生成**：创建带时间戳的 SQL 文件
6. **替换**：可选地将原始中文替换为 `module.hash` 格式

## 开发

```bash
# 运行测试
go test ./...

# 构建
go build -o i18n-trans ./cmd/i18n-trans

# 本地运行
go run ./cmd/i18n-trans scan -d ./test
```

## 许可证

MIT
