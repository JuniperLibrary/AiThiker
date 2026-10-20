# prompt-engineering -- 提示工程

**路径:** 	opics/prompt-engineering/
**作用:** 研究指令设计、Few-shot 选择、System Prompt 编写、输出格式控制等提示工程技术。

## 关键文档

| 文件 | 内容 |
|------|------|
| index.md | 提示工程全景与范式 |
| system-prompt-design.md | 系统提示的结构化编写方法 |
| ew-shot-selection.md | 示例选择策略（KNN、语义聚类、动态选择）|
| output-formatting.md | JSON mode、Structured Output、Schema 绑定 |
| prompt-catalog/ | 可复用的提示模板目录 |

## 提示模板规范

所有提示模板**必须**使用语言标记为 prompt 的 fenced 代码块:

`prompt
你是一个任务规划助手。你的职责是:
1. 分析用户输入的意图
2. 将复杂任务分解为子步骤
3. 为每个子步骤分配合适的工具

输出格式要求:
- 步骤编号从 1 开始
- 每个步骤包含: 步骤号、描述、所需工具
`

### 提示分类
- **System Prompts:** 定义角色、行为边界、安全约束
- **Task Prompts:** 面向具体任务的指令
- **Format Prompts:** 控制输出结构和格式
- **Correction Prompts:** 错误修复和重新生成

## 搜索指引

- prompt-catalog/ 中的 .md 文件: 即用型提示模板
- index.md 的 **Patterns** 章节: 高频提示模式
- 跨话题引用: ../llm-reasoning/chain-of-thought.md

## 质量检查

- [ ] 每个提示模板使用 prompt 代码块标记
- [ ] 包含输入/输出示例
- [ ] 注明适用场景和局限性
- [ ] 模板支持参数化替换