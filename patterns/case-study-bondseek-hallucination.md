---
title: BondSeek 防幻觉体系（自反思三重循环 + 引用治理全解析）
tags: [case-study, hallucination, self-reflection, sufficiency-check, citation, faithfulness, rag, golang]
created: 2026-10-21
updated: 2026-10-21
status: draft
---

# BondSeek 防幻觉体系（自反思三重循环 + 引用治理全解析）

> **案例对象:** [bondseek-ai](https://github.com/) — 债券行业多租户 AI SaaS 投研平台（Go 1.24 微服务）
> **分析范围:** 防幻觉的完整实现：自反思三重循环（充分性 / 幻觉检测 / 有用性）+ 两种幻觉检测模式（Deep 逐句 / Fast 轻量）+ 引用治理（生成时强制引用规则 + 生成后引用清洗）
> **资料代码:** `bondseek-common/rag/`（reflector/reflector.go 574 行 + generator/generator.go 791 行 + service.go 自反思编排 + prompts/prompts.go 检测提示词）（2026-10-21 代码级深挖）
> **关联文档:** [RAG 检索系统](./case-study-bondseek-rag.md)、[多 Agent 编排](./case-study-bondseek-multi-agent.md)
> **面试话题索引:** [agent-interview-questions.md](../notes/agent-interview-questions.md) — 第 11 题"如何降低模型幻觉"（本文即答案素材）

## 📌 名词速查表（先看这里）

> 本文讲防幻觉，核心是"自反思三重循环 + 引用"。先看这里（完整对照见文末"名词标注表"）：

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **Hallucination** | "幻觉" | 模型一本正经地编造不存在的"事实" |
| **Self-Reflection** | "自反思" | 模型生成后自己当"质检员"，检查自己答得怎么样 |
| **Sufficiency** | "充分性" | 检索到的资料够不够回答问题 |
| **Faithfulness** | "忠实度" | 回答里的每一句是不是都有原文依据 |
| **Citation** | "引用/出处" | 回答里标注的资料来源标记 `[report:xxx_0]` |
| **Coverage** | "覆盖率" | 有多少比例的句子带了引用 |
| **Threshold** | "阈值" | 及格线（超过/低于就触发重试） |
| **Regenerate** | "重新生成" | 检测不合格，带着"警告"再生成一遍 |
| **Supplement** | "补充检索" | 资料不够，再查一次补信息 |
| **Deep 模式** | "深度质检" | 逐句检查（最严格，最贵） |
| **Fast 模式** | "快速质检" | 一次性整体检查（便宜，够用） |

---

## 🎯 TL;DR 冲突摘要（先看这段）

> **这篇文档解决的核心矛盾：模型生成回答的自信程度和它的正确程度完全无关——它不知道自己不知道。** 金融场景下，模型编一个"某券收益率 3.2%"可能就是一次亏钱决策的依据，幻觉不可接受。BondSeek 的破局思路一句话总结：**"把'生成'和'质检'分成两个环节，让模型自己当自己的质检员——先检查资料够不够（充分性），再检查每句话有没有出处（幻觉检测），最后检查答案有没有用（有用性），不合格就带警告重来。"** 读完全文你只需要带走一个画面：**防幻觉 = 生成之后的"三道质检关卡"，每道关卡都是"采样一个分数 → 和阈值比 → 不合格就返工"，外加"引用"从生成时强制、到生成后清洗的全程治理。**

---

## 1. 问题域：为什么金融 RAG 必须防幻觉

### 1.1 幻觉的三种经典成因（理解全文的钥匙）

| 成因 | 通俗解释 | 对应防线 |
|------|---------|---------|
| **检索不给力** | 资料库里根本查不到答案，模型只能"编" | 充分性检查 + 补充检索 |
| **生成不忠实** | 资料里有答案，但模型润色时自己发挥加了料 | 幻觉检测 + 重新生成 |
| **信息过时** | 查到了但资料过期，答案正确但无效 | 时效性维度（充分性评分） |

### 1.2 为什么"生成后检查"比"生成前预防"更重要

- **生成前**能做的是：给好上下文、写好 prompt、让模型强制带引用——但这些都**治标**，模型仍可能出错
- **生成后**是最后一道闸：**它检查的是"结果"，而不是"过程"**——过程不可控，结果可量化（有阈值、有分数、可判断 pass/fail）
- BondSeek 的做法：**生成前预防（引用规则）+ 生成后质检（自反思三重循环），两道防线叠加**

---

## 2. 自反思三重循环：完整编排（service.go:1001-1137）⭐

### 2.1 三循环总览

```
检索完成 ──▶ 【循环 1：充分性】资料够不够？
                │  SUFFICIENT → 继续
                │  PARTIAL/INSUFFICIENT → GapQuery 补充检索（≤MaxSupplementRounds轮）
                ▼
             生成回答（Generate / MapReduce，强制带引用）
                ▼
             【循环 2：幻觉】每句话有依据吗？
                │  幻觉率 ≤ 阈值 → 继续
                │  幻觉率 > 阈值 → 带"警告语"重新生成（≤MaxRegenerateRounds轮）
                ▼
             【循环 3：有用性】答案有用吗？
                │  FinalScore ≥ 最低分 → success
                │  FinalScore < 最低分 → status="partial" + 提示补充信息
                ▼
             返回（Answer + confidence 置信度 + status）
```

### 2.2 循环 1：信息充分性检查（service.go:1001-1047）

```go
for round := 0; round < s.config.MaxSupplementRounds; round++ {
	suffResult, err := s.reflector.CheckSufficiency(ctx, req.Query, retrieveResult)
	if suffResult.Judgment == "SUFFICIENT" {
		break                     // ✅ 够用，出循环
	}
	if suffResult.GapQuery != "" && round < MaxSupplementRounds-1 {
		// 用"缺口问题"再检一次（TopK 减半，主问题优先）
		supplementReq := &QueryRequest{Query: suffResult.GapQuery, TopK: req.TopK / 2, ...}
		supplementResult := s.retriever.Retrieve(ctx, supplementReq, nil)
		retrieveResult = s.mergeRetrieveResults(retrieveResult, supplementResult)  // 合并
	}
}
```

**关键机制：**
- LLM 对检索结果打 **5 维评分**（每维 0-2 分）：核心信息 / 细节深度 / 时效性 / 多角度 / 可信度
- 总分 0-10：0-4=INSUFFICIENT，5-7=PARTIAL，8-10=SUFFICIENT
- 不足时 **LLM 自己生成 `gap_query`**（"还缺什么"），用它再做一次**补充检索**，结果与原有合并
- **TopK/2**：补充检索只取一半候选——防止补充内容淹没主结果
- **循环次数受限**（MaxSupplementRounds），防止无限补查

> **设计要点**：充分性检查发生在**生成之前**——与其等模型编完再发现错误，不如先确认"弹药够不够"。这是"**先问够不够，再开枪**"哲学。

### 2.3 循环 2：幻觉检测 + 重新生成（service.go:1071-1109）

```go
for round := 0; round < s.config.MaxRegenerateRounds; round++ {
	hallResult, err := s.reflector.CheckHallucination(ctx, answer, retrieveResult.Chunks)
	hallucinationRate = hallResult.HallucinationRate
	passed := hallucinationRate <= s.reflector.GetConfig().HallucinationThreshold

	if passed { break }   // ✅ 幻觉率达标

	// ❌ 不达标：带着"警告"重新生成
	warningMsg := fmt.Sprintf(
		"请注意：以下句子可能存在事实错误：%v。请严格依据原文重新生成回答。",
		hallResult.ProblematicSentences)   // ← 把问题句子喂回去！
	genResult, err = s.generator.Generate(ctx, req.Query+" "+warningMsg, ...)
	answer = genResult.Answer
}
```

**关键机制：**
- **幻觉率** = 问题句子数 / 总句子数（reflector 里用 badCount/len(sentences) 计算）
- **阈值比对**：`hallucinationRate <= HallucinationThreshold`（默认见 constants）→ 通过
- **重新生成不是"从头再来"**：把"疑似幻觉的句子列表"拼进 query 再生成一次——**带着质检结果的返工**（不是盲目重试）
- **循环次数受限**（MaxRegenerateRounds）——不能无限重试

> **设计要点**：这是"**检测-反馈-重做**"的闭环。它比"直接重试"聪明在：把**具体的错误句子**告诉模型，让它知道错在哪、照着原文改。

### 2.4 循环 3：有用性评估（service.go:1111-1137）

```go
usefulResult, err := s.reflector.CheckUsefulness(ctx, req.Query, answer)
if usefulResult.FinalScore < s.reflector.GetConfig().MinUsefulnessScore {
	status = "partial"
	missingInfo = usefulResult.SupplementQuery   // 告诉用户"我还缺这块，你可以补充"
}
confidence = float64(usefulResult.FinalScore) / 5.0   // 置信度（返回给前端）
```

**关键机制：**
- **FinalScore 1-5 分**：站在用户角度评"这答案到底帮没帮你解决问题"
- **分数不够 → `status="partial"`**：不隐藏问题，把 `SupplementQuery`（建议用户补充什么）返回给前端——**诚实告知不完整**
- **分数转置信度**：`confidence = FinalScore/5`（0.2-1.0），随回答一起返回——**下游可以用它做风险提示**

> **设计要点**：有用性评估把"质量"变成**可返回的元数据**（confidence / status / missingInfo），让**调用方（前端/其他 Agent）**能根据置信度决定怎么展示——这是"质检结果要可消费"的思想。

---

## 3. 幻觉检测：Deep 逐句 vs Fast 轻量（reflector.go）

### 3.1 双模式对比

| 维度 | Deep 逐句（CheckHallucination） | Fast 轻量（CheckHallucinationLight） |
|------|------------------------------|-------------------------------------|
| 调用次数 | **每句一次 LLM**（N 句 = N 次） | **整体 1 次 LLM** |
| 检查粒度 | 拆句 → 每句核对引用 → 逐句验证 | 整段回答一次性对照原文 |
| 成本 | 高（N 次调用） | 低（1 次） |
| 返回 | 逐句 judgment + 幻觉率 | 全局 faithfulness_score + 无依据声明列表 |
| 适用 | Deep 模式（自反思循环 2） | Fast 模式（轻量检测） |
| 兜底 | 调用失败该句跳过 | 解析失败默认 MOSTLY_FAITHFUL(0.8) |

### 3.2 Deep 逐句检测的三段式（reflector.go:203-279）

```
Step 1 拆句 + 提取引用（extractSentencesWithCitations）
        └─ 正则按 。！？ 拆句，提取每句的 [source_type:chunk_id] 引用标记
Step 2 核对引用：三类问题句
        ├─ 无引用（NO_CITATION）→ 直接记差（除非是连接句）
        ├─ 引用无效（INVALID_CITATION）：chunkMap 里查不到 → 记差
        └─ 引用有效 → 进入 Step 3 逐句 LLM 验证
Step 3 逐句验证（checkSingleSentence）：句子 + 引用 + 原文 → LLM 判定
        └─ FULLY_SUPPORTED / PARTIALLY_SUPPORTED / NO_SUPPORT（NO_SUPPORT 记差）
最终 幻觉率 = 问题句子数 / 总句子数
```

**良心细节：** 连接性句子（"综上所述""因此"）**不算事实性陈述，免检**（isConnectiveSentence）——避免把过渡句误判为幻觉。

### 3.3 Fast 轻量检测（reflector.go:398-445）

- 一次性提交：整个回答 + 生成时使用的**原始上下文**（`ChunksContext`，generator 特意返回的字段）
- LLM 判级：FAITHFUL / MOSTLY_FAITHFUL / PARTIALLY_FAITHFUL / UNFAITHFUL（对应分数区间）
- 返回 `unsupported_claims`（最多 5 条无依据陈述）——便于后续定位

---

## 4. 引用治理：从"生成时强制"到"生成后清洗"（generator.go）⭐

### 4.1 引用是防幻觉的第一道闸

生成 prompt 强制模型**每句带引用**（`[source_type:chunk_id]` 格式），如：`**来源 [report:doc_0]**\n内容...`。这样：
- **生成时**：模型被迫"看着原文说"，凭空发挥的空间被压缩
- **生成后**：每个句子都有"身份证明"，幻觉检测才能逐句核对

### 4.2 生成后引用清洗（extractAndCleanCitations，generator.go:363-423）

LLM 经常产出"脏引用"，需要代码清洗：

| 脏引用类型 | 示例 | 处理 |
|-----------|------|------|
| **占位符拼接**（把 prompt 字面量拼进引用） | `[source_type:chunk_id:report:xxx_0]` | 正则还原为 `[report:xxx_0]` |
| **Entity 自创标签** | `[entity:xxx]` | 删除 |
| **无效格式**（缺 source_type） | `[a1b2c3-chunk-4]` | 删除 |
| **非法 source_type** | `[foo:xxx]` | 删除（只认 report/news/kb） |
| **引用了不存在的 chunk** | `[report:不存在的id]` | 删除 |

```go
// validSourceTypes：只认这三种，其余一律清除
var validSourceTypes = map[string]bool{
	"report": true, "news": true, "kb": true,
}
// 只保留：①合法格式 ②存在对应 chunk 的引用 → 生成 SourceCitation 列表
```

**为什么这段代码重要**：LLM 生成的引用里，**只要有"-"出现错误，整个引用链（展示→点击→溯源）就断了**。清洗保证**引用要么有效、要么从回答里删除**——绝不留"看似有出处、实际查不到"的假引用。

> **设计要点**：引用清洗是"确定性代码治理 LLM 输出"的又一实例——**模型的引用习惯不可靠，代码负责把它约束成可靠格式**（呼应总纲哲学一：LLM 不可信 → 规则兜底）。

---

## 5. 防幻觉体系的整体心智模型

### 5.1 三层防线图

```
第一层 检索层：三路召回 + 充分性检查（资料够不够）      ← 治"无米之炊"
第二层 生成层：强制引用 + MapReduce（只基于事实）       ← 治"自由发挥"
第三层 质检层：幻觉检测 + 有用性评估（结果达不达标）     ← 治"漏网之鱼"
        └─ 全部失败兜底：status/confidence 如实上报（不装成功）
```

### 5.2 为什么这套设计最值得学

| 设计 | 学什么 |
|------|--------|
| **把"生成"和"质检"分开** | 让模型既当选手又当裁判（自反思）——比你"相信它一次生成"可靠得多 |
| **检测结果反馈回生成** | 返工不是盲目的，是"带着具体错误清单"的返工 |
| **分数+阈值+循环上限** | 每个环节都可量化、可截止——不会无限烧钱 |
| **引用全程治理** | 强制生成时带 → 生成后清洗 → 质检时核对，引用是贯穿全程的"事实锚点" |
| **如实上报不完美** | 答得不好就 status=partial + 建议补充，不假装成功 |

### 5.3 面试回答框架（30 秒版）

> "我们降低幻觉分三个层面。**检索层**：RAG 检索后先做**充分性检查**——让 LLM 给资料打 5 维分（核心信息/细节/时效/多角度/可信度），不足 8 分就生成补充查询再检一次，把缺口补上。**生成层**：强制模型每个事实句带引用标记，只认 report/news/kb 三种来源，引用格式用代码清洗，无效引用从回答里删掉。**质检层**：Deep 模式做**逐句幻觉检测**——把回答拆成句子，每句检查和引用原文的支撑关系，算幻觉率；超标就把'疑似问题句子'反馈给模型重新生成；最后还有**有用性评估**，分数不够就如实标记 status=partial 并提示缺什么。整个过程分数都带阈值、循环都有上限，成本可控。"

---

## 反模式（本主题避免）

- **只在生成前预防**——prompt 写得再好模型也会出错，必须有生成后的质检闸
- **盲目重试**——重新生成必须带"具体错误句子"，否则重试多少次都一样
- **假引用**——引用要么有效、要么删掉，绝不能留有标记但查不到原文的"假出处"
- **无限循环**——充分性/幻觉检测都有轮数上限，防止无限烧钱
- **隐藏不完美**——回答质量差就如实 status=partial + 给建议，不装成功
- **过度检测**——连接句不算事实陈述，别把过渡句误判成幻觉（免检白名单）

---

## 📖 名词标注表（完整中英对照）

> 本文（防幻觉体系）全部关键英文术语，看懂这张表 = 读懂本文。

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **Hallucination** | 幻觉 | 模型生成无依据的内容（编造事实） |
| **Faithfulness** | 忠实度 | 回答内容对原文的忠实程度 |
| **Self-Reflection** | 自反思 | 模型生成后自检（当前题的核心机制） |
| **Sufficiency Check** | 充分性检查 | 判断检索资料是否足够回答 |
| **SUFFICIENT / PARTIAL / INSUFFICIENT** | 足够 / 部分 / 不足 | 充分性三档判断 |
| **Gap Query** | 缺口问题 | 资料缺什么，LLM 生成的补充检索问句 |
| **补充检索 (Supplement)** | 补查 | 用缺口问题再检一次，结果合并 |
| **Merge 合并** | 结果融合 | 将补充检索结果并入原结果集 |
| **Hallucination Rate** | 幻觉率 | 问题句子数 / 总句子数 |
| **Threshold** | 阈值 | 通过/不通过的分数线 |
| **Regenerate** | 重新生成 | 检测不合格，带警告重做 |
| **Problematic Sentences** | 问题句子 | 被判定为幻觉的句子列表 |
| **Usefulness** | 有用性 | 回答对用户的实际帮助程度（1-5分） |
| **Confidence** | 置信度 | 有用性分数转成的 0.2-1.0 数值 |
| **status** | 状态标记 | success / partial（部分有用） |
| **Citation** | 引用/出处 | `[report:xxx_0]` 格式的来源标记 |
| **SourceCitation** | 结构化引用 | 带 SourceType/DocID/ChunkID 的引用对象 |
| **Coverage Rate** | 引用覆盖率 | 有引用的句子比例（零 LLM 计算） |
| **ChunksContext** | 生成上下文 | 生成时用的原始 chunk 文本（供质检复用） |
| **逐句检测 (Sentence-level)** | 句子级质检 | 每句一次 LLM 检查 |
| **整体检测 (Answer-level)** | 回答级质检 | 整段一次 LLM 检查 |
| **FULLY / PARTIALLY / NO_SUPPORT** | 完全支撑 / 部分 / 无支撑 | 单句验证的三种判断 |
| **INVALID_CITATION** | 无效引用 | chunkMap 里查不到对应内容 |
| **连接句 (Connective)** | 过渡句 | "综上所述"等，免检的事实性豁免 |
| **MapReduce** | 映射-归约 | 分块提取事实→汇总生成（防长文幻觉） |
| **JSON 安全规则** | 输出保护 | 提示词里禁未转义双引号，用「」防解析失败 |

---

## 关联文档

- [`case-study-bondseek-rag.md`](./case-study-bondseek-rag.md) — RAG 检索（本文是检索结果的质检环节）
- [`case-study-bondseek-multi-agent.md`](./case-study-bondseek-multi-agent.md) — 多 Agent 编排（SubAgent 产出也走质检）
- [`../topics/agent-architecture/from-zero-to-agent.md`](../topics/agent-architecture/from-zero-to-agent.md) — 总纲（哲学一：LLM 不可信 → 规则兜底）
- [`notes/agent-interview-questions.md`](../notes/agent-interview-questions.md) — 面试话题索引（第 11 题）
