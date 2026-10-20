# Safety and Alignment — 目录约定

**路径:** topics/safety-and-alignment/
**作用:** Agent 系统约束执行、输出过滤、价值对齐与红队测试。

## 文档分类

| 类别 | 命名模式 | 说明 |
|------|----------|------|
| 入口 | index.md | 话题总览（已存在） |
| 约束机制 | constraint-*.md | 硬约束与软约束实现 |
| 对齐方法 | alignment-*.md | RLHF、DPO、Constitutional AI 等 |
| 红队测试 | redteam-*.md | 对抗性测试方法与工具 |
| 案例分析 | case-*.md | 安全失效事件复盘 |
| 实验记录 | YYYY-MM-DD-*.md | 安全实验记录 |

## 风险评估表格约定

每个安全威胁文档必须包含风险评估表：

| 威胁类型 | 严重程度 | 发生概率 | 检测难度 | 缓解措施 |
|----------|----------|----------|----------|----------|
| Prompt Injection | Critical | High | Medium | 输入净化 + 权限隔离 |
| Data Leakage | Critical | Medium | High | 输出扫描 + 隐私过滤 |
| Jailbreak | High | Medium | Medium | 对抗训练 + 一致性检查 |
| Tool Misuse | High | Low | Low | 沙箱执行 + 参数校验 |
| Value Misalignment | Medium | Low | High | RLHF + 宪法 AI |

## 对齐税分析约束

评估安全机制时必须附带对齐税（Alignment Tax）分析：

| 方案 | 安全增益 | 能力损失 | 延迟影响 | 综合对齐税 |
|------|----------|----------|----------|------------|
| 方案 A | +90% | -5% | +10ms | 低 |
| 方案 B | +95% | -15% | +50ms | 中 |
| 方案 C | +99% | -30% | +200ms | 高 |

## 反模式（本目录外）

- 只谈安全不谈代价 → 每个安全机制必须附面对齐税分析
- 忽视失效模式 → 讨论安全机制的失效边界与降级策略
- 单一攻击向量 → 红队测试必须覆盖多类别攻击面

---

交叉引用: → ../prompt-engineering/, ../evaluation-and-benchmarks/, ../fine-tuning-for-agents/
