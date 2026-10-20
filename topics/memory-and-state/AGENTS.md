# Memory and State — 目录约定

**路径:** topics/memory-and-state/
**作用:** Agent 记忆管理与状态持久化 — 短期记忆、长期记忆、情景记忆。

## 文档类型

| 类型 | 命名模式 | 说明 |
|------|----------|------|
| 入口 | index.md | 话题概述与索引（已存在） |
| 记忆类型 | memory-*.md | 各记忆类型深度分析 |
| 检索策略 | retrieval-*.md | RAG、MMR、HyDE 等检索方法 |
| 会话压缩 | session-*.md | 会话压缩策略、生命周期与持久化 |
| 序列化 | serialization-*.md | 状态存储格式与恢复机制 |
| 实验记录 | YYYY-MM-DD-*.md | 记忆系统实验 |

## 记忆对比表约定

所有记忆类型分析必须使用格式统一的对比表：

| 维度 | 工作记忆 | 长期记忆 | 情景记忆 |
|------|----------|----------|----------|
| 存储介质 | Context Window | Vector Store | Experience Buffer |
| 容量 | ~128K tokens | 无上限 | 可配置 |
| 持久性 | 会话级别 | 跨会话 | 跨会话 |
| 检索方式 | Sliding Window | Embedding Similarity | Temporal Recall |
| 典型实现 | GPT-4 context | Chroma / Pinecone | Replay Buffer |

## 检索策略评估表

| 算法 | 精度 | 多样性 | 延迟 | 适用场景 |
|------|------|--------|------|----------|
| Cosine Top-K | 高 | 低 | 低 | 精确匹配 |
| MMR | 中 | 高 | 中 | 多样性需要 |
| HyDE | 高 | 中 | 高 | 查询模糊 |

## 反模式（本目录避免）

- 记忆与检索耦合 — 记忆存储与检索策略拆分为独立文档
- 不做容量分析 — 每种存储方案必须标注容量上界与退避策略
- 忽略遗忘机制 — 记忆系统必须包含过期/淘汰策略

---

引用外部话题: 见 ../agent-architecture/, ../patterns/, ../tool-use/
