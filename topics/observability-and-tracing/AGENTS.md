# 可观测性与链路追踪 —— 领域约定

**领域:** 可观测性与链路追踪
**类型:** 💡 补充
**目录:** `topics/observability-and-tracing/`
**入口文件:** `index.md`

## 领域范围

本话题关注 Agent 系统的可观测性设计，涵盖行为监控、多步链路追踪、性能分析、日志格式、可视化方案以及调试工具。

## 文档分类

| 类别 | 命名模式 | 示例 |
|------|----------|------|
| 追踪架构 | tracing-*.md | tracing-distributed-agent.md |
| 日志格式 | logging-*.md | logging-structured-agent.md |
| 可视化 | visualization-*.md | visualization-trace-tree.md |
| 性能分析 | performance-*.md | performance-latency-bottleneck.md |
| 监控告警 | monitoring-*.md | monitoring-agent-health.md |

## 内容规范

### Trace 格式约定

所有 trace 数据使用标准化的 JSON 结构描述，遵循 OpenTelemetry 语义约定：

```json
{
  "trace_id": "abc123def456",
  "span_id": "span-001",
  "parent_span_id": null,
  "name": "agent.execute_tool",
  "start_time": "2026-08-13T10:00:00.000Z",
  "end_time": "2026-08-13T10:00:01.200Z",
  "duration_ms": 1200,
  "status": "ok | error | timeout",
  "attributes": {
    "agent.id": "agent-alpha",
    "tool.name": "web_search",
    "tool.params": "{\"query\": \"latest AI papers\"}",
    "llm.model": "gpt-4o",
    "llm.tokens": 450
  },
  "events": [
    {
      "timestamp": "2026-08-13T10:00:00.050Z",
      "name": "tool.call.start",
      "attributes": {}
    }
  ]
}
```

### 日志级别约定

| 级别 | 用途 | 示例场景 |
|------|------|----------|
| DEBUG | 调试信息 | 每次 LLM 调用的完整 Prompt |
| INFO | 关键节点 | Agent 状态变更、工具调用完成 |
| WARN | 异常但不中断 | 重试、降级、超时警告 |
| ERROR | 失败但可恢复 | 工具调用失败、API 错误 |
| FATAL | 不可恢复 | Agent 崩溃、安全违规 |

### 链路可视化

多步 Agent trace 使用时序图或树形图表示，关键路径标注耗时与状态。

### 性能基准

每个 Agent 性能分析必须记录：P50/P95/P99 延迟、成功率、平均 Token 消耗。

## 相关话题

- `../agent-architecture/` — Agent 运行时可观测集成
- `../evaluation-and-benchmarks/` — 性能评估与基准
- `../tool-use/` — 工具调用追踪

---

> *本文档为领域约定，用于统一话题下的文档编写规范。*