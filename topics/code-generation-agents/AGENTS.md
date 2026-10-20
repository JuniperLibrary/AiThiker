# 代码生成 Agent —— 领域约定

**领域:** 代码生成 Agent
**类型:** 🔷 进阶
**目录:** `topics/code-generation-agents/`
**入口文件:** `index.md`

## 领域范围

本话题关注利用 LLM 进行代码合成的技术，涵盖单函数生成、仓库级代码生成、self-debugging、代码评估与集成模式。

## 文档分类

| 类别 | 命名模式 | 示例 |
|------|----------|------|
| 生成策略 | generation-*.md | generation-repo-level.md |
| 调试修复 | debugging-*.md | debugging-self-heal.md |
| 评估方法 | evaluation-*.md | evaluation-pass-at-k.md |
| 集成模式 | integration-*.md | integration-ide-plugin.md |
| 安全考量 | security-*.md | security-code-injection.md |

## 内容规范

### 代码示例格式

所有代码示例必须同时展示生成前后的变化，使用 diff 格式标注：

```diff
- // 原代码（被替换的部分）
+ // 生成的代码（新添加的部分）
```

较大的文件级变更，需分别展示原文件和生成后文件，并在文件头部标注路径。

### 评估指标定义

| 指标 | 缩写 | 说明 |
|------|------|------|
| Pass@k | p@k | 生成 k 个候选，至少一个通过测试的概率 |
| Functional Correctness | FC | 生成代码正确实现功能的比率 |
| Syntax Validity | SV | 生成代码语法合法的比率 |
| Test Coverage | TC | 生成代码对测试用例的覆盖率 |

### 生成上下文格式

仓库级生成的上下文信息必须结构化描述：

```json
{
  "repository": "repo-name",
  "target_file": "src/service.ts",
  "dependencies": ["module-a", "module-b"],
  "existing_types": ["TypeA", "TypeB"],
  "generation_strategy": "fill-in-the-middle | scratch | edit"
}
```

### 实验复现

每个实验记录必须包含：模型名称、温度参数、采样数量 k、测试集名称。

## 相关话题

- `../llm-reasoning/` — 推理驱动的代码生成
- `../evaluation-and-benchmarks/` — 代码评估基准
- `../tool-use/` — 代码执行工具调用

---

> *本文档为领域约定，用于统一话题下的文档编写规范。*