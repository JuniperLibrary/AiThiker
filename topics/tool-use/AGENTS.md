# tool-use -- 工具使用与函数调用

**路径:** topics/tool-use/
**作用:** 探讨 Agent 如何定义、发现、调用工具，以及处理调用过程中的错误和异常。

## 关键文档

| 文件 | 内容 |
|------|------|
| index.md | 工具使用架构总览 |
| tool-definition.md | Function Calling Schema 设计（计划，未创建） |
| tool-registry.md | 工具注册、发现、版本管理（计划，未创建） |
| invocation-patterns.md | 同步/异步/流式调用、并行调用、重试语义（主张-证据矩阵 + 失效边界卡 + N=20 实测 + 三层面试回答） |
| error-handling.md | 工具调用失败的处理策略（降级、重试、报错）（计划，未创建）|

## JSON Schema 规范

所有工具定义**必须**使用 JSON Schema 格式的 fenced 代码块:

`json
{
  "name": "search_knowledge_base",
  "description": "在知识库中搜索相关文档",
  "parameters": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "搜索查询语句"
      },
      "top_k": {
        "type": "integer",
        "description": "返回结果数量",
        "default": 5
      }
    },
    "required": ["query"]
  }
}
`

### 工具注册模式
- **静态注册:** 预定义工具列表，启动时加载
- **动态发现:** Agent 运行时通过描述获取可用工具
- **分层注册:** 基础工具 -> 复合工具 -> Agent 专用工具

## 搜索指引

- tool-registry.md 中的 **Implementation** 章节: 注册模式代码
- invocation-patterns.md 中的 **Examples**: 调用示例
- 跨话题引用: ../agent-architecture/cognitive-architecture-paradigms.md（原 cognitive-loop.md 合并至此文档的 ReAct 章节）

## 质量检查

- [ ] 每个工具定义附带 JSON Schema
- [ ] 包含至少一种调用模式的代码示例
- [ ] 错误处理覆盖超时、限流、无效参数场景
- [ ] 工具描述清晰、参数命名语义化