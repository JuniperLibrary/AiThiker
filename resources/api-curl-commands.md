# API 测试 curl 命令集

> **来源:** `test-curl.sh`（SIT 环境实测参数, 2026-08-12）
> **用途:** 快速调用内网模型 API 的 curl 命令参考

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
> ⚠️ **Qwen 直连报 `ModelArts.8902, Api key auth failed`** — 说明文档中的 QWEN_KEY 可能不正确（或与 GLM_KEY 搞反了）。
> 建议先用 GLM_KEY 的值调 Qwen 试试能否通，或者从 Postman 重新截图核对 Key 中的模糊字符。

---

## 直连上游（不经 proxy）

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
