# 工具使用与函数调用

> Agent 如何发现、选择、调用工具并处理工具执行结果。

## 内容覆盖

- 工具定义模式（JSON Schema、TypeScript 类型）
- 工具选择与路由策略
- 参数填充与验证
- 工具执行与错误处理
- 工具链与组合调用

## 目录内文档

| 文档 | 主题 |
|------|------|
| [`function-calling.md`](./function-calling.md) | **函数调用原理**：工具三要素（定义/参数/结果）+ 完整调用闭环 + 工程坑（防死循环/错误处理/上下文污染）+ BondSeek Go 落地实例 |

## 核心资源

- `prompt-engineering/` — 工具调用提示
- `agent-architecture/` — 工具使用在架构中的位置
- `experiments/` — 工具使用实验

---

*本目录为话题入口，细分文档用英文连字符命名。*
