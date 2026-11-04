# planning-and-decomposition -- 任务规划与分解

**路径:** topics/planning-and-decomposition/
**作用:** 研究 Agent 的任务规划与分解策略，包括层级规划、依赖解析、动态重规划等技术。

## 关键文档

| 文件 | 内容 |
|------|------|
| index.md | 规划与分解的全景图 |
| hierarchical-planning.md | HNN、HTN、自上而下/自下而上分解（计划，未创建） |
| dependency-resolution.md | 任务依赖图、拓扑排序、并行化分析（计划，未创建） |
| replanning.md | 动态重规划、执行中断恢复、计划修正（计划，未创建） |
| plan-representation.md | 计划的表示格式（线性、DAG、树形）（计划，未创建）|

## 计划结构规范

所有规划示例**必须**使用 YAML 格式的 fenced 代码块:

`yaml
plan:
  goal: "为用户生成一份市场分析报告"
  steps:
    - id: 1
      description: "收集原始数据"
      depends_on: []
      tool: data_collector
      fallback: "使用缓存数据"

    - id: 2
      description: "清洗与预处理"
      depends_on: [1]
      tool: data_cleaner

    - id: 3
      description: "生成分析报告"
      depends_on: [2]
      tool: report_generator
      output: report.md
`

### 分解策略
- **线性分解:** 顺序执行，链式依赖
- **并行分解:** 独立子任务并发执行
- **条件分解:** 根据中间结果动态分支
- **递归分解:** 子任务再分解为更小的步骤

## 搜索指引

- plan-representation.md 中的 **Formats** 章节: 不同表示格式对比
- replanning.md 中的 **Triggers** 章节: 触发重规划的典型场景
- 跨话题引用: ../tool-use/invocation-patterns.md（计划，未创建）

## 质量检查

- [ ] 每个规划策略附带 YAML 格式的计划示例
- [ ] 依赖关系清晰（depends_on 显式声明）
- [ ] 包含 fallback 或重规划策略
- [ ] 对比不同分解策略的复杂度与适用场景