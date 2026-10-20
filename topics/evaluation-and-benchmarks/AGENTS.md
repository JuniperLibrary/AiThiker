# Evaluation and Benchmarks — 目录约定

**路径:** topics/evaluation-and-benchmarks/
**作用:** Agent 能力评估方法、基准测试套件与对抗性测试。

## 文档分类

| 类别 | 命名模式 | 说明 |
|------|----------|------|
| 入口 | index.md | 话题总览（已存在） |
| 基准套件 | benchmark-*.md | 特定基准详细分析 |
| 评估方法 | method-*.md | 评估指标与框架设计 |
| 对抗测试 | adversarial-*.md | 对抗性测试方法论 |
| 实验记录 | YYYY-MM-DD-*.md | 评估实验结果 |

## 基准结果约定

所有基准对比必须包含 Effect Size，格式统一。

| 模型/系统 | GAIA (Pass@1) | SWE-bench (Resolved) | AgentBench (Avg) | Effect Size vs Baseline |
|-----------|---------------|---------------------|------------------|------------------------|
| Baseline | 22.5% | 18.3% | 41.2 | — |
| System A | 31.8% | 26.7% | 52.6 | d=0.73 (Medium) |
| System B | 38.2% | 34.1% | 58.4 | d=1.12 (Large) |
| System C | 42.5% | 39.8% | 63.7 | d=1.48 (Large) |

Effect Size 标准: d<0.2 无, 0.2<=d<0.5 小, 0.5<=d<0.8 中, d>=0.8 大

## 多维度评估模型

每个评估设计方案应包含多维度分解：

| 评估维度 | 指标 | 方法 | 权重 |
|----------|------|------|------|
| 任务完成度 | Pass@K, Success Rate | 端到端执行 | 40% |
| 效率 | 成本, 延迟, Token 消耗 | 运行时日志 | 20% |
| 鲁棒性 | 扰动容忍度 | 对抗性测试 | 20% |
| 安全性 | 违规率, 拒绝正确率 | 红队测试 | 20% |

## 反模式（本目录外）

- 无 effect size 的对比 → 性能对比表需标注统计显著性
- 数据污染未声明 → 注意基准套件是否已泄露到训练数据
- 单一指标冒充综合评分 → 多维度分解展示

---

交叉引用: → ../agent-frameworks/, ../safety-and-alignment/, ../experiments/
