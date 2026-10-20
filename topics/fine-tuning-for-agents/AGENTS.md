# Agent 微调技术 —— 领域约定

**领域:** Agent 微调技术
**类型:** 🔷 进阶
**目录:** `topics/fine-tuning-for-agents/`
**入口文件:** `index.md`

## 领域范围

本话题关注针对 Agent 任务的模型微调技术，涵盖监督微调（SFT）、RLHF、DPO、工具调用训练、训练数据生成与微调后评估。

## 文档分类

| 类别 | 命名模式 | 示例 |
|------|----------|------|
| 方法综述 | method-*.md | method-dpo-for-agents.md |
| 数据构建 | data-*.md | data-synthetic-generation.md |
| 训练配方 | recipe-*.md | recipe-tool-use-sft.md |
| 评估分析 | eval-*.md | eval-post-fine-tuning.md |
| 框架工具 | framework-*.md | framework-axolotl-config.md |

## 内容规范

### 训练配置格式

所有训练配方必须使用 YAML 格式描述超参数配置：

```yaml
# recipe: tool-use-sft
model:
  base: "Qwen2.5-7B"
  dtype: bfloat16

training:
  epochs: 3
  learning_rate: 2e-5
  optimizer: adamw_torch
  lr_scheduler: cosine
  warmup_ratio: 0.1
  per_device_batch_size: 4
  gradient_accumulation: 8
  deepspeed: zero2

data:
  format: sharegpt
  path: "data/tool-use-sft.jsonl"
```

### 数据格式约定

训练数据统一使用以下对话格式：

```json
{
  "messages": [
    { "role": "system", "content": "系统指令" },
    { "role": "user", "content": "用户请求" },
    { "role": "assistant", "content": "模型回复（含工具调用）" }
  ],
  "tools": [{ "name": "search", "description": "..." }]
}
```

### 评估指标

微调后评估必须报告以下指标：

| 指标 | 说明 |
|------|------|
| Tool Call Accuracy | 工具调用参数正确率 |
| Task Success Rate | 整体任务成功率 |
| Instruction Following | 指令遵循率 |
| Hallucination Rate | 幻觉/错误工具调用率 |

### 消融实验

超参数消融实验使用表格对比，标注最佳配置。

## 相关话题

- `../tool-use/` — 工具调用训练数据设计
- `../evaluation-and-benchmarks/` — 微调后评估方法
- `../llm-reasoning/` — 推理能力微调

---

> *本文档为领域约定，用于统一话题下的文档编写规范。*