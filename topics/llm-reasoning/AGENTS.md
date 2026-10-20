# llm-reasoning -- LLM 推理机制

**路径:** 	opics/llm-reasoning/
**作用:** 分析 LLM 的推理机制，包括 Chain-of-Thought、Tree-of-Thought、Self-Consistency、Reflection 等技术。

## 关键文档

| 文件 | 内容 |
|------|------|
| index.md | 推理机制总览与分类 |
| chain-of-thought.md | CoT 提示、Zero-shot CoT、Auto-CoT |
| 	ree-of-thought.md | ToT 搜索策略（BFS/DFS）、评估函数 |
| self-consistency.md | 多路径采样与投票聚合 |
| eflection.md | 自我反思、错误修正、Critique 模式 |
| experiments/ | 本地实验记录（位于 experiments/reasoning/）|

## 推理示例规范

所有推理示例**必须**包含清晰的步骤标记，使用 > **Step N:** 格式:

> **Step 1: 理解问题**
> 将用户输入解析为结构化目标。
>
> **Step 2: 分解子目标**
> 将大问题拆分为可独立处理的步骤。
>
> **Step 3: 逐步推理**
> 每一步输出中间结果，作为下一步输入。
>
> **Step 4: 验证与修正**
> 检查最终答案是否满足原始约束。

## 搜索指引

- **论文引用:** 查看 esources/reasoning-papers.md
- **实验记录:** experiments/reasoning/ 下的日期前缀 .md 文件
- **实现对比:** index.md 中的 Comparison 表格

## 质量检查

- [ ] 每个推理方法包含至少一个带步骤标记的示例
- [ ] 对比不同方法的优劣势
- [ ] 实验记录附带超参数配置
- [ ] 指向 esources/ 中的原始论文