# Patterns — Agent 设计模式

**路径:** `patterns/`
**作用:** 记录可复用的 Agent 设计模式、架构模板和常见问题的标准化解决方案。

## 范围

- 认知架构模式（ReAct、Plan-and-Execute、Reflection 等）
- 通信模式（Agent-to-Agent、Human-in-the-Loop）
- 错误恢复与容错模式
- 状态管理与持久化模式

## 约定

- 每个模式一个独立文件: `pattern-name.md`
- 包含：问题描述 → 解决方案 → 权衡分析 → 代码示例
- 与 `topics/` 中的相关话题交叉引用
- 真实项目案例分析使用前缀: `case-study-项目名-主题.md`

## 内容索引

| 文件 | 主题 |
|------|------|
| [`case-study-bondseek-memory.md`](./case-study-bondseek-memory.md) | BondSeek 生产级记忆与上下文系统：三层记忆、质量闸写入、双轨存储、冷热遗忘 |
