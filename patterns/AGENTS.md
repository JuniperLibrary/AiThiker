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
| [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) | BondSeek 用户记忆（L1 画像）专项：tag 枚举、explicit/inferred 分层、dedup 博弈、触发时机 |

## 关联矩阵（理论 ↔ 案例 ↔ 框架）

> 阅读案例时，对照本矩阵快速定位对应的理论知识章节。

| 案例中的设计 | 对应的理论知识 | 对应框架概念 |
|-------------|--------------|-------------|
| **双轨存储**（展示历史永不压缩 vs 工作上下文随时可压） | [`memory-and-state/context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) §3.2 分层组装 | LangGraph Checkpoint（持久化状态） |
| **两阶段渐进压缩**（50% 剪枝 / 70% 压缩） | [`memory-and-state/session-compression-in-agent-systems.md`](../topics/memory-and-state/session-compression-in-agent-systems.md) §1.2 渐进式压缩 | LangGraph 状态裁剪 |
| **L1 用户画像注入 system prompt**（XML 全量注入） | [`context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) §4.2 生命周期·召回 | LangChain Memory 类（外挂记忆） |
| **四道质量闸写入**（失败整批放弃） | [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) §3 提取管线 | mem0 阈值过滤 |
| **冷热遗忘**（60/120 天物理删索引） | [`context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) §4.2 生命周期·遗忘 | 无直接对应（业界弱项） |
| **触发治理**（仅 user 触发，cron 跳过） | [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) §8 触发时机 | 无直接对应（业务定制） |
