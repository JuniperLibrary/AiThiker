# API 测试 curl 命令集

> **来源:** `test-curl.sh`（SIT 环境实测参数, 2026-08-12）
> **用途:** 快速调用内网模型 API 的 curl 命令参考

## 📌 名词速查表（先看这里）

> 本篇是接口测试命令集，命令参数术语多。核心词先看这里（完整对照见文末"名词标注表"）：

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **API** | "接口" | 程序间通信的通道（这篇是模型服务接口） |
| **curl** | "看-URL" | 一条在终端发网络请求的命令工具 |
| **HTTP** | 网络协议 | 浏览器/接口传输数据的标准规则 |
| **endpoint / 端点** | "接口地址" | 具体调用哪个地址（如 /v1/chat/completions） |
| **header / 请求头** | "头信息" | 请求的附加信息（如身份凭证 Authorization） |
| **Authorization / Bearer** | "认证/令牌" | 身份凭证：`Bearer <钥匙>` 表示"我是这把钥匙的主人" |
| **request body / body** | "请求体" | 请求里携带的正文内容（JSON 数据） |
| **stream** | "流式开关" | `true`=边生成边推送，`false`=一次性返回 |
| **proxy** | "代理" | 中间转发层（本例指 127.0.0.1:8005） |
| **Postman** | 接口调试工具（专有名词） | 图形化发送请求、调试接口的软件 |

---

## 环境参数

| 参数 | 值 |
|------|-----|
| 上游网关 | `20.200.25.216:8080` |
| 协议 | HTTP (OpenAI / FastChat 兼容) |
| Body 特殊字段 | `chatId`, `detail` |

---

## 模型凭证

| 模型 | 全称 | API Key |
|------|------|---------|
| **GLM** | `service-GLM-5-w8a8-vllm-Bigep` | `NnVkBHYB7SkS9fXWnvOytael_Rf46j_ijen2h7iH4Zt_by3857pDx26s3igrMYS19CzCI26UHUYTLCQ6U5GvBw` |
| **Qwen** | `service-Qwen35-397B-A17B-W8a8-test` | `N9r_TGRoGAsM0Z7-R5kbbhWRpZjmZpLsXid1LVjdj_wp55LosTK9PKgv4MLHLT321JskOqLNbCPczwfDSy3nQ` |

> ⚠️ Key 从 Postman 截屏辨认，个别字符（`l/I/1`、`O/0`）待文本核对。
>
> ⚠️ **Qwen 直连报 `ModelArts.8902, Api key auth failed`（密钥认证失败）** — 说明文档中的 QWEN_KEY 可能不正确（或与 GLM_KEY 搞反了）。
> 建议先用 GLM_KEY 的值调 Qwen 试试能否通，或者从 Postman 重新截图核对 Key 中的模糊字符。

---

## 直连上游（不经 proxy）

> **端点（endpoint）说明:** `POST http://20.200.25.216:8080/v1/chat/completions`
> - `chat/completions` = "聊天补全"接口：给模型发对话，得到回复
> - `-H "Authorization: Bearer <Key>"` = 带身份令牌请求
> - `-d '{...}'` = 发送请求体（JSON 数据）
> - `-m 60` = 最多等 60 秒；`-s` = 静默（不显示进度条）；`-N` = 流式（边收边显示）

### GLM 非流式

```bash
curl -s -m 60 "http://20.200.25.216:8080/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer NnVkBHYB7SkS9fXWnvOytael_Rf46j_ijen2h7iH4Zt_by3857pDx26s3igrMYS19CzCI26UHUYTLCQ6U5GvBw" \
  -d '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
      { "content": "你是谁", "role": "user" }
    ],
    "model": "service-GLM-5-w8a8-vllm-Bigep",
    "max_tokens": 16,
    "stream": false
  }' | head -c 800
```

### GLM 流式

```bash
curl -sN -m 60 "http://20.200.25.216:8080/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer NnVkBHYB7SkS9fXWnvOytael_Rf46j_ijen2h7iH4Zt_by3857pDx26s3igrMYS19CzCI26UHUYTLCQ6U5GvBw" \
  -d '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
      { "content": "你是谁", "role": "user" }
    ],
    "model": "service-GLM-5-w8a8-vllm-Bigep",
    "max_tokens": 16,
    "stream": true
  }'
```

### Qwen 非流式

```bash
curl -s -m 60 "http://20.200.25.216:8080/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer N9r_TGRoGAsM0Z7-R5kbbhWRpZjmZpLsXid1LVjdj_wp55LosTK9PKgv4MLHLT321JskOqLNbCPczwfDSy3nQ" \
  -d '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
      { "content": "你是谁", "role": "user" }
    ],
    "model": "service-Qwen35-397B-A17B-W8a8-test",
    "max_tokens": 16,
    "stream": false
  }' | head -c 800
```

### Qwen 流式

```bash
curl -sN -m 60 "http://20.200.25.216:8080/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer N9r_TGRoGAsM0Z7-R5kbbhWRpZjmZpLsXid1LVjdj_wp55LosTK9PKgv4MLHLT321JskOqLNbCPczwfDSy3nQ" \
  -d '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
      { "content": "你是谁", "role": "user" }
    ],
    "model": "service-Qwen35-397B-A17B-W8a8-test",
    "max_tokens": 16,
    "stream": true
  }'
```

---

## 走 proxy（假设 proxy 127.0.0.1:8005）

> proxy 会覆写 `Authorization` header，key 用 `placeholder` 即可。

### 健康检查

```bash
curl -s "http://127.0.0.1:8005/healthz"
```

### 列模型列表

```bash
curl -s -m 5 "http://127.0.0.1:8005/glm/v1/models" -H "Authorization: Bearer placeholder"
curl -s -m 5 "http://127.0.0.1:8005/qwen/v1/models" -H "Authorization: Bearer placeholder"
```

### GLM 走 proxy

```bash
curl -s -m 60 "http://127.0.0.1:8005/glm/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer placeholder" \
  -d '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
      { "content": "你是谁", "role": "user" }
    ],
    "model": "service-GLM-5-w8a8-vllm-Bigep",
    "max_tokens": 16,
    "stream": false
  }' | head -c 800
```

### Qwen 走 proxy

```bash
curl -s -m 60 "http://127.0.0.1:8005/qwen/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer placeholder" \
  -d '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
      { "content": "你是谁", "role": "user" }
    ],
    "model": "service-Qwen35-397B-A17B-W8a8-test",
    "max_tokens": 16,
    "stream": false
  }' | head -c 800
```

---

## 快速参考表

| 场景 | 模型 | API Key | 端点 |
|------|------|---------|------|
| GLM 直连 | `service-GLM-5-w8a8-vllm-Bigep` | GLM_KEY | `20.200.25.216:8080/v1/chat/completions` |
| Qwen 直连 | `service-Qwen35-397B-A17B-W8a8-test` | QWEN_KEY | `20.200.25.216:8080/v1/chat/completions` |
| GLM 走 proxy | 同上 | placeholder | `127.0.0.1:8005/glm/v1/chat/completions` |
| Qwen 走 proxy | 同上 | placeholder | `127.0.0.1:8005/qwen/v1/chat/completions` |
| 健康检查 | — | — | `127.0.0.1:8005/healthz` |
| 列模型(GLM) | — | placeholder | `127.0.0.1:8005/glm/v1/models` |
| 列模型(Qwen) | — | placeholder | `127.0.0.1:8005/qwen/v1/models` |

---

## Request Body 结构

```json
{
  "chatId": "abcdefghrweqw12679001112291111",
  "detail": false,
  "messages": [
    { "content": "你是谁", "role": "user" }
  ],
  "model": "<模型全称>",
  "max_tokens": 16,
  "stream": false
}
```

> `chatId` 为上游网关自定义字段（非标准 OpenAI 字段）；`detail` 控制是否返回详细信息。
>
> **字段逐个解释（方便不熟英文的读者）：**
> - `chatId` = 会话标识：同一 ID 的多次请求会被网关视为"同一段对话"
> - `detail` = 详细信息开关：`true` 返回更多元数据
> - `messages` = 消息数组：给模型的对话历史（可多条）
> - `role` = 消息角色：`user` 用户 / `system` 系统 / `assistant` 助手
> - `model` = 模型名称：指定用哪个模型（GLM 或 Qwen 全称）
> - `max_tokens` = 最大输出量：模型这次最多生成多少个词元
> - `stream` = 流式开关：`true` 边生成边返回，`false` 一次性返回

---

## Postman 导入说明

> **文档中的 bash curl 命令（含 `| head -c`、`\` 换行、单引号 body）Postman 无法解析。**
> 下方提供 Postman 原生 `--location` 格式，直接复制 → Import → Raw text 粘贴即可导入。

### GLM 直连（Postman 可导入）

```bash
curl --location --request POST 'http://20.200.25.216:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer NnVkBHYB7SkS9fXWnvOytael_Rf46j_ijen2h7iH4Zt_by3857pDx26s3igrMYS19CzCI26UHUYTLCQ6U5GvBw' \
--data-raw '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
        {
            "content": "你是谁",
            "role": "user"
        }
    ],
    "model": "service-GLM-5-w8a8-vllm-Bigep",
    "max_tokens": 16,
    "stream": false
}'
```

### Qwen 直连（Postman 可导入）

```bash
curl --location --request POST 'http://20.200.25.216:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer N9r_TGRoGAsM0Z7-R5kbbhWRpZjmZpLsXid1LVjdj_wp55LosTK9PKgv4MLHLT321JskOqLNbCPczwfDSy3nQ' \
--data-raw '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
        {
            "content": "你是谁",
            "role": "user"
        }
    ],
    "model": "service-Qwen35-397B-A17B-W8a8-test",
    "max_tokens": 16,
    "stream": false
}'
```

### GLM 流式（Postman 可导入）

```bash
curl --location --request POST 'http://20.200.25.216:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer NnVkBHYB7SkS9fXWnvOytael_Rf46j_ijen2h7iH4Zt_by3857pDx26s3igrMYS19CzCI26UHUYTLCQ6U5GvBw' \
--data-raw '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
        {
            "content": "你是谁",
            "role": "user"
        }
    ],
    "model": "service-GLM-5-w8a8-vllm-Bigep",
    "max_tokens": 16,
    "stream": true
}'
```

### Qwen 流式（Postman 可导入）

```bash
curl --location --request POST 'http://20.200.25.216:8080/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer N9r_TGRoGAsM0Z7-R5kbbhWRpZjmZpLsXid1LVjdj_wp55LosTK9PKgv4MLHLT321JskOqLNbCPczwfDSy3nQ' \
--data-raw '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
        {
            "content": "你是谁",
            "role": "user"
        }
    ],
    "model": "service-Qwen35-397B-A17B-W8a8-test",
    "max_tokens": 16,
    "stream": true
}'
```

### GLM 走 proxy（Postman 可导入）

```bash
curl --location --request POST 'http://127.0.0.1:8005/glm/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer placeholder' \
--data-raw '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
        {
            "content": "你是谁",
            "role": "user"
        }
    ],
    "model": "service-GLM-5-w8a8-vllm-Bigep",
    "max_tokens": 16,
    "stream": false
}'
```

### Qwen 走 proxy（Postman 可导入）

```bash
curl --location --request POST 'http://127.0.0.1:8005/qwen/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer placeholder' \
--data-raw '{
    "chatId": "abcdefghrweqw12679001112291111",
    "detail": false,
    "messages": [
        {
            "content": "你是谁",
            "role": "user"
        }
    ],
    "model": "service-Qwen35-397B-A17B-W8a8-test",
    "max_tokens": 16,
    "stream": false
}'
```

**导入步骤：**
1. 复制上方任意一段 curl 命令
2. Postman → **Import** → **Raw text** → 粘贴 → **Continue** → **Import**
3. Postman 自动解析出 URL、Headers、Body

---

## 📖 名词标注表（完整中英对照）

> 本篇（API 测试命令集）全部关键英文术语，看懂这张表 = 读懂本文。

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **API** (Application Programming Interface) | 接口 | 程序之间通信的约定通道（这篇是模型服务接口） |
| **curl** | 命令行网络工具 | 在终端一键发起 HTTP 请求的工具 |
| **HTTP** (HyperText Transfer Protocol) | 网页传输协议 | 浏览器/接口传输数据的标准规则 |
| **URL** | 网址/接口地址 | 具体访问哪个地址，如 `http://20.200.25.216:8080/...` |
| **endpoint** | 端点/接口路径 | URL 中具体调用的路径（如 `/v1/chat/completions`） |
| **网关 (gateway)** | 统一入口 | 多个请求都先经过它转发，统一鉴权（本文 `:8080`） |
| **proxy** | 代理 | 中间转发层，收到请求再转给上游（本文 `:8005`） |
| **Authorization** | 授权/认证头 | 请求头中的身份凭证字段 |
| **Bearer** | 令牌类型 | 认证方式：`Bearer <Key>` = "持有这把钥匙的人" |
| **API Key** | 密钥/钥匙 | 验证身份的密码，谁持有谁就能调用 |
| **header** | 请求头 | 请求的附加信息（内容类型、身份凭证等） |
| **Content-Type** | 内容类型 | 告诉服务端"我发的是 JSON 数据" |
| **request body** | 请求体 | 请求携带的正文数据（JSON） |
| **JSON** | 数据格式（专有名词） | 结构化文本格式 `{"key":"value"}` |
| **messages** | 消息数组 | 给模型的对话历史（可多条 user/system/assistant） |
| **role** | 角色 | 消息身份：user 用户 / system 系统 / assistant 助手 |
| **model** | 模型 | 指定调用哪个模型（填全称） |
| **max_tokens** | 最大输出词元数 | 限制模型这次最多生成多少内容 |
| **stream** | 流式开关 | `true` 流式逐段返回 / `false` 一次返回 |
| **chatId** | 会话标识 | 同 ID 的多次请求视为同一段对话（本网关自定义字段） |
| **detail** | 详细信息开关 | `true` 返回更多元数据 |
| **三星模型: GLM / Qwen** | 模型名（专有名词） | 两个可用的内网模型（智谱 GLM / 通义 Qwen） |
| **vLLM / Bigep / W8a8** | 部署技术代号 | 模型服务部署框架与量化方案（不必深究） |
| **healthz** | 健康检查接口 | 检测服务是否活着：返回 OK 即正常 |
| **/v1/models** | 模型列表接口 | 查询该服务下有哪些可用模型 |
| **placeholder** | 占位符 | 假钥匙：proxy 会改写身份，随便填即可 |
| **Postman** | 接口调试工具（专有名词） | 图形化发送请求/调试接口的桌面软件 |
| **Import / Raw text** | 导入 / 原文粘贴 | Postman 的导入方式：直接粘贴 curl 命令 |
| **bash** | 终端脚本语言 | Linux/Windows 终端里的命令执行环境 |
| **flags（-s -m -N -H -d）** | 命令参数开关 | `-s` 静默 / `-m 60` 超时 / `-N` 流式 / `-H` 加头 / `-d` 发 body |
| **SIT 环境** | 系统集成测试环境 | 上线前专门做联调测试的环境 |
