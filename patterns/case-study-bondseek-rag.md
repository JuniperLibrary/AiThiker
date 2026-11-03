---
title: BondSeek RAG 检索系统（Fast 模式全流程分析）
tags: [case-study, rag, retrieval, query-expansion, rrf, rerank, time-decay, golang]
created: 2026-10-21
updated: 2026-10-21
status: draft
---

# BondSeek RAG 检索系统（Fast 模式全流程分析）

> **案例对象:** [bondseek-ai](https://github.com/) — 债券行业多租户 AI SaaS 投研平台（Go 1.24 微服务）
> **分析范围:** RAG 检索链路的完整实现：Query 扩展 / 三路并行召回 / PG 补全 / RRF 融合 / Rerank 精排 / 时间衰减 / 分区裁剪；Fast 与 Deep 模式区分；三种检索入口
> **资料代码:** `bondseek-common/rag/`（retriever/retriever.go 1051 行 + query_processor.go 579 行 + graph_traverser.go + storage/ + constants/rag_types.go）（2026-10-21 代码级深挖）
> **关联文档:** [整体案例分析](./case-study-bondseek-memory.md)、[用户记忆专项](./case-study-bondseek-user-memory.md)、[会话压缩专题](../topics/memory-and-state/session-compression-in-agent-systems.md)

## 📌 名词速查表（先看这里）

> 本文涉及检索系统全链路，英文术语最多最密。核心词先看这里（完整对照见文末"名词标注表"）：

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **RAG** | "检索增强生成" | 先检索相关文档片段，再让 LLM 基于片段回答（解决幻觉） |
| **Query** | "查询/问题" | 用户问的一句话，检索管线的输入 |
| **Retrieve** | "检索" | 从数据库里把相关片段找出来 |
| **Chunk** | "分块/片段" | 文档被切成的小块（每条可独立检索） |
| **Entity / Relation** | "实体 / 关系" | 知识图谱里的"节点"和"连线"（如"贵州茅台"→"控股"→"茅台集团"） |
| **Embedding** | "向量化/嵌入" | 把文本变成一串数字，语义相近的数字也相近 |
| **Rerank** | "重排序" | 用更聪明的模型把候选按相关性重新排一遍 |
| **RRF** | "倒数排名融合" | 把多路排序结果合并成一路的数学方法 |
| **TopK** | "前 K 条" | 只取相似度最高的前 K 个结果 |
| **Filter** | "过滤条件" | 检索前先按条件筛掉不可能的（如时间段、来源） |

---

## 🎯 TL;DR 冲突摘要（先看这段）

> **这篇文档解决的核心矛盾：你想让模型"答得准、答得新、答得快"——但这三件事互相打架。** 答得准要召回足够多候选（多路检索），召回多了又费 token、放不进窗口；答得新要有时效性，但 3 个月前的研报依然有参考价值不能一刀砍；答得快要求管线短，但每个增强环节（Query 扩展、Rerank、图谱）都要多几次模型调用。BondSeek 的破局思路一句话总结：**"用 LLM 的聪明换查询的准（费模型但答得对），再用确定性规则保性能和安全（分区、衰减、过滤全是死代码）。"** 读完全文你只需要带走一个画面：**检索 = 先"广撒网"（三路并行召回），再"择优录取"（RRF 合并 + Rerank 精排），全程"定时剪枝"（时间裁剪）——每一层都在回答一个问题："候选够不够多？结果够不够好？窗口放不放得下？"**

---

## 1. 检索架构总览：三种入口 + 两种模式

### 1.1 存储层（五类数据库分工）

| 存储 | 角色 | 存什么 |
|------|------|--------|
| **Milvus** | 向量库 | chunk / entity / relation 的向量 + 标量过滤字段（按月分区） |
| **PostgreSQL (PG)** | 事实主库 | chunk 正文内容、实体描述、关系描述、证据映射（entity↔chunk） |
| **NebulaGraph** | 图数据库 | 实体与关系的图结构（Deep 模式多跳遍历用） |
| **ClickHouse / Redis** | 辅助 | 统计 / 缓存（本文不展开） |

> **核心分工思想**：**向量只负责"找候选"，正文永远回 PG 取。** Milvus 里不存大文本，只存向量 + ID + 过滤字段——检索完再按 ID 回 PG 拿完整内容。

### 1.2 三种检索入口（`Retrieve` 方法，retriever.go:120）

```go
func (r *Retriever) Retrieve(ctx, req, cbs) (*RetrieveResult, error) {
	if req.SourceFilter.IsSingleDoc() || req.SourceFilter.IsSingleFile() {
		result, err := r.retrieveSingleDoc(ctx, req)   // 入口1：单文档
		if estimateChunkTokens(result.Chunks) > r.config.SingleDocMaxTokens { // 超 6 万 token
			return r.retrieveNormal(ctx, req, cbs)     //   自动回退标准检索
		}
		return result, nil
	}
	return r.retrieveNormal(ctx, req, cbs)             // 入口2：标准（Fast/Deep）
	// 入口3：SimpleChunkSearch —— 轻量版，跳过扩展/实体/图谱
}
```

| 入口 | 触发场景 | 走什么路径 |
|------|---------|-----------|
| `retrieveSingleDoc` | 用户指定"就在这一篇/这一个文件里查"（如某份研报的问答） | 单路：Embedding → Milvus chunk(带 source/file 过滤) → PG 补全；超 60k token 自动回退标准 |
| `retrieveNormal` | 全库/公共/混合检索（最常用） | Fast：扩展 → 三路并行 → 补全 → RRF → Rerank；Deep 再加图遍历 |
| `SimpleChunkSearch` | 轻量场景（无需复杂检索） | 单路：Embedding → Milvus chunk → 补全 → Rerank，跳过全部扩展 |

### 1.3 Fast vs Deep 模式（constants/rag_types.go:12-16）

| 维度 | **Fast（默认）** | **Deep** |
|------|----------------|---------|
| Query 扩展 | ✅ 相同 | ✅ 相同 |
| 三路并行 Milvus | ✅ 相同 | ✅ 相同 |
| RRF + Rerank | ✅ 相同 | ✅ 相同 |
| NebulaGraph 多跳遍历 | ❌ 不做 | ✅ `req.Mode == QueryModeDeep` 时执行 |
| 典型场景 | 实时对话、流式问答（低延迟） | 复杂交叉问题（"XX 的关联方有哪些风险"） |

> **延迟 vs 深度的权衡**：Fast 是检索管线的"保底方案"，延迟可控；Deep 走图遍历，每跳都调 LLM 决策，适合需要"关系推理"的问题。代码里 Deep 的图遍历失败**不阻塞**主链路（`logger.Warn` 后继续返回 Fast 结果）——**增强永远不拖垮主路径**。

---

## 2. Query 处理器：四步扩展管线（query_processor.go）

> **Query 处理器** = 在真正检索前，先用 LLM 把"模糊的一句话"变成"能命中资料的多句话"。Fast 和 Deep 共用。

### 2.1 管线全貌（`ExpandQuery`，query_processor.go:260）

```
原始查询 "它最近涨了多少？"
   │ Step 0: Rewrite 重写（指代消解 + 时间规范化 + 关键词，单次 LLM 调用）
   ▼
  "贵州茅台近 3 个月股价涨幅？"  + TimeFilter{start: 2026-07-21}
   │ Step 1: Step-Back 抽象（退一步问宏观问题）
   ▼
  "2026 年 Q3 白酒板块整体走势与贵州茅台的市场表现"
   │ Step 2: Decompose 分解（最多 5 个子问题）
   ▼
  ["茅台近期股价与涨跌幅", "白酒板块 Q3 行情", "茅台近期重大事件"]
   │ Step 3: Multi-Query（每个子问题并行生成最多 3 个查询变体）
   ▼
  ExpandedQueries 合并 → 去重 → 输出候选查询池（供后续检索/未来扩展）
```

### 2.2 Step 0：Rewrite——指代消解 + 时间规范化（最关键的一步）

- **为什么必须做**："它""上个月""最近"这些都是**相对表达**，不消解就没法检索
- **单次 LLM 调用**同时输出三样：`resolved_query`（消解后文本）+ `time_filter`（结构化的起止时间）+ `high/low_level_keywords`（关键词）
- **即使没有历史也要做**（`buildQueryRewritePrompt` 注释：没有历史对话也要执行时间规范化）——因为"最近"必须锚定到当前时间
- **全部 `enable_thinking: false`**（关闭思考模式）：预处理任务要的就是**低延迟**，不要模型纠结
- **失败降级**：LLM 挂掉/解析失败 → 返回原始查询（不阻塞管线）

```json
// LLM 返回的 JSON（parseQueryRewriteResult 定义）
{
  "resolved_query": "贵州茅台近3个月股价涨幅",
  "time_filter": {"start": "2026-07-21", "end": ""},
  "high_level_keywords": ["白酒板块", "贵州茅台", "股价涨幅"],
  "low_level_keywords": ["600519", "贵州茅台股价", "涨幅排行"]
}
```

### 2.3 Step 1~3：三步扩展（提高召回率）

| 步骤 | 目的 | 关键参数 |
|------|------|---------|
| **Step-Back** | 从具体问题"退一步"生成宏观背景问题（语境兜底） | 1 个抽象问题 |
| **Decompose** | 复杂问题拆成子问题（分而治之） | `MaxSubQuestions=5` |
| **Multi-Query** | 每个子问题再生成多种问法（同义扩展） | `MaxQueryVariants=3`，**并行**生成 |

- Decompose 时把 step-back 结果作为"全局上下文"传入（`stepBackContext`），让子问题不跑偏
- Multi-Query 用 `sync.WaitGroup` 对全部子问题**并发**重写（query_processor.go:309-331）
- 最终 `ExpandedQueries` 去重，**目前只用了前两个**（见 §2.4 埋点）

### 2.4 ⚠️ 代码埋点：关键词抽取结果目前没用上

```go
// retriever.go:170-175 —— 关键发现
keywords, err := r.queryProcessor.ProcessQuery(ctx, expansionResult.ResolvedQuery)
if err != nil {
    logger.Warn("process query failed", zap.Error(err))
    keywords = &types.ExtractedKeywords{LowLevel: []string{expansionResult.ResolvedQuery}}
}
_ = keywords   // ← 抽取出了关键词，但被丢弃了！
```

**`ProcessQuery` 单独又做了一次关键词抽取（还要调一次 LLM！），结果 `_ = keywords` 直接丢弃。** 组合推理：

- 这 + 额外的 LLM 调用（成本）+ 埋点编码方式（`_ =` 是有意的"先占位"写法）→ **多半是为未来 Hybrid Search（混合检索：向量 + 关键词/BM25）预留的接口**
- **面试可答点**：看到无法解释的代码不要慌，从"埋点特征"（无用但存在的调用、占位赋值、常量预留）反推设计意图
- **代价**：当前每轮检索 Baseline 就有 2+ 次 LLM 预处理调用（Rewrite × 1 + 关键词抽取 × 1 + Step-Back/Decompose/Multi-Query 若干）

---

## 3. Fast 检索全流程（retriever.go:151-412）

### 3.1 十步流程图

```
Step 1  Query 扩展（ExpandQuery：Rewrite + Step-Back + Decompose + Multi-Query）
Step 2  关键词抽取（ProcessQuery ── 埋点，结果未使用）
Step 3  对 ResolvedQuery 生成 Embedding
Step 4  三路并行 Milvus 搜索（errgroup 并发）
        ├─ Route 1: chunk 向量搜索（来源多样性分桶 report:15/news:10/kb:10）
        ├─ Route 2: entity 向量搜索（Top20）
        └─ Route 3: relation 向量搜索（Top20，无实体过滤）
Step 5  实体过滤的关系搜索（用 Route 2 的实体 ID 再搜一次关系 → 提高图谱关联质量）
Step 6  合并关系结果 + 提取全部关联实体 ID
Step 7  并行 PG 补全（entity 描述 / relation 描述 / 证据 chunk_ids，errgroup 并发）
Step 8  RRF 融合（向量 chunk 候选 + 图谱证据 chunk_ids，k=60）
Step 9  Rerank 精排（交叉编码器重排，取 Top30）
Step 10 PG 补全 chunk 正文 → 返回 RetrieveResult
        └─（Deep 模式在此追加 NebulaGraph 多跳遍历）
```

### 3.2 Step 4：三路并行 Milvus（检索的"招数组合"）

用 `errgroup` 并发发三路搜索（retriever.go:198-236），**任何一路失败只记日志不影响其他路**（`g.Go` 内 `logger.Warn` 后 `return nil`）：

| 路 | 搜什么 | 过滤 | TopK | 目的 |
|----|--------|------|------|------|
| Route 1 | **chunk**（文档片段） | 权限+来源+时间（见 §4） | 20（分桶后） | 找到"说了相关内容的原文" |
| Route 2 | **entity**（实体） | 权限过滤 | 20 | 找到"提到谁"（茅台/中信/城投） |
| Route 3 | **relation**（关系） | 权限过滤 | 20 | 找到"谁和谁有什么关系" |

> **为什么三路都要**：向量搜索只认"字形相似"，债券问题经常是"A 公司债券是否由 B 担保"——纯 chunk 搜不到这种跨越两段文字的关系，必须靠图谱。三路是**召回策略的互补**：文本相似度 + 实体命中 + 关系命中。

### 3.3 Step 5~7：图谱向事实的"补全与搭桥"

- **Step 5**：拿 Route 2 的实体 ID 作为"种子实体"，再搜一次**限定这些实体的关系**（`SearchRelationsWithEntityFilter`）——让关系落在"已经提到过的实体"上，过滤噪声
- **Step 6**：合并两路关系结果（去重按 RelationID），并从关系里提取源/目标实体 ID，得到**全量实体集合**
- **Step 7**：并行三项 PG 查询（retriever.go:285-319）：
  1. `GetEntityDescriptions`：实体的规范名/描述（回填到 entity 结构体，补 CanonicalName/Description 空缺）
  2. `GetRelationDescriptions`：关系的描述文本
  3. `GetChunkIDsByEvidence`：**查"哪些 chunk 提到了这些实体/关系"** —— 这就是把图谱检索结果"翻译回"文本证据的关键桥梁

> **设计洞见**：Milvus 里 entity/relation 向量是"索引"，PG 是"事实"；而 **evidence（证据）映射**（entity↔chunk）让图谱检索结果能反查出对应的文本片段，从而进入 Rerank 竞争。图谱不是检索终点，是通往文本的中转站。

### 3.4 Step 8：RRF 融合（Reciprocal Rank Fusion）

```go
// retriever.go:681-711 —— 倒数排名融合
func rrfFusion(rankedLists [][]string, k int) []string {
	scores := make(map[string]float64)
	for _, list := range rankedLists {
		for rank, id := range list {
			scores[id] += 1.0 / float64(rank+k)   // 名次越好，贡献越大
		}
	}
	// 按总得分降序 → 返回融合后的 ID 列表
}
```

- **融合的两路**：① 向量检索得到的 chunk 候选（`vectorChunkIDs`）② 图谱证据 chunk（`evidenceChunkIDs`）——**注意是两路，不是三路**（entity/relation 不直接进融合，而是通过证据映射"化身"为 chunk 参与）
- **公式**：每个 chunk 的得分 = Σ 1/(rank + k)，k=60。在每路里排第 1 名贡献 1/61，第 30 名贡献 1/90——**名次越靠前权重越大，但不会一票否决**
- **为什么用 RRF 而非加权求和**：向量相似度分数和图谱证据分数**量纲不同、不可比**，直接加权重拍脑袋；RRF 只用"名次"不用"原始分数"，天然归一化（归一化=把不同尺度的数据拉到同一量级），是融合异构召回结果的工业标准做法

### 3.5 Step 9：Rerank 精排（交叉编码器）

- **为什么需要**：向量检索的相似度是"粗筛"，Rerank 用**交叉编码器**（把 query 和 chunk 拼在一起让模型打分）做"精排"——相关性判断质量高一个档次
- **流程**：先从 PG 取 fusion 结果的正文 → `rerankChunks`（retriever.go:650）把 query + 全部候选发给 Rerank API → 按返回结果重新排序 → 截断取 Top30（`RerankTopN=30`）
- **失败降级**：Rerank 挂了 → 用 RRF 融合结果的原顺序（`logger.Warn` 后走 Step 10）
- **候选池 vs 最终数**：RRF 后的候选可能几十上百条，Rerank 后只留 30 条进上下文——**多召回、精排后馈送给 LLM**

### 3.6 Step 10：结果组装

- 用融合后的 chunk ID 从 PG 拿完整正文，组装 `RetrieveResult{Chunks, Entities, Relations, ResolvedQuery}`
- **回调**（`RetrieveCallbacks`）：每一阶段进度（OnQueryRewrite / OnEmbeddingDone / OnVectorSearchDone / OnRerankDone / OnGraphTraverse）都推给调用方——用于前端流式展示检索阶段

---

## 4. safeSearch：分区裁剪 + 时间衰减 + 软分批（retriever.go:827-902）

> Milvus 全库扫描会 OOM（内存爆掉），BondSeek 用 **partition_key（分区键）按月切表** + 三层防护（见下）。

### 4.1 三种 DataScope 的处理

| DataScope | 处理方式 |
|-----------|---------|
| **private**（私人） | **固定走 PRIVATE 分区**，不带任何时间/月份/衰减筛选（用户自己的文档不看时间） |
| **public**（公共） | 按 `timeFilterToMonths` 生成月份列表 → 只搜这些月份的分区 |
| **all/混合** | 公共月份分区 + PRIVATE 分区一起搜 |

### 4.2 三层时间防护（BondSeek 最值得抄的安全设计）

1. **partition_key 月份裁剪**（粗筛）：`timeFilterToMonths` 把 TimeFilter 转成 `["202607","202608","202609"]` 等月份列表，filter 里加 `publish_year_month in [...]`——**先按分区砍掉不可能的时间段**。LLM 没识别出时间 → 默认回看 3 个月（`defaultPublicMonthsBack=3`）
2. **publish_date 月内精筛**（细筛）：月份命中后再在 partition 内按 `publish_date >= start AND publish_date <= end` 过滤（支持"7 月 15 日到 8 月 15 日"这种跨月但不到整月的查询）
3. **软分批**（防 OOM）：月份跨度 ≤ `maxPartitionsPerQuery=24`（2 年）走单次查询；**超过 24 个月 → 按 24 个月切批并发查，全局合并后按相似度截断**（`chunkStringSlice` + errgroup）

### 4.3 Decay Ranker：时间衰减（新闻/研报 vs 知识库区别对待）

```go
// retriever.go:1040-1050 —— exp 指数衰减函数（origin=当前时间）
const (
	decayOffsetSeconds = 7 * 24 * 60 * 60  // 近 7 天满分
	decayScaleSeconds  = 30 * 24 * 60 * 60 // 衰减尺度 30 天
	decayValue         = 0.5               // 37 天时权重 50%
)
```

| 数据源 | 是否衰减 | 理由 |
|--------|---------|------|
| **private（私人文档）** | ❌ 不衰减 | 用户的资料不随时间过时 |
| **kb / conversation**（知识库/对话） | ❌ 不衰减 | 知识库无时效性（如评级方法论） |
| **news / report / 混合** | ✅ exp 衰减 | 金融新闻/研报**越新越相关** |

- **衰减曲线与直观感受**：7 天内满分 → 37 天 50% → 67 天 25% → 97 天 12.5%
- **为什么用 exp 不用 gauss**（代码注释明确写了）：gauss 在 67 天权重只剩 ~0.06（断崖），exp 在 3 个月后仍有 ~12.5%——**研报 3 个月前的依然有参考价值，不该直接归零**（金融长尾诉求）
- **Milvus Function API 实现**：`WithType(FunctionTypeRerank)` + `WithParam("reranker","decay")`，作为 Milvus 内置 rerank 器传进搜索
- **每人一个新的 origin**：`newDecayRanker()` 每次构造新对象，避免多查询共用一个时间原点导致衰减失真

> **面试可答点**：时间衰减不是"一刀切加权重"，而是**按业务场景区别对待**——私人/知识库内容是"事实型"，新闻/研报是"时效型"，这是金融检索与通用检索的核心差异。

---

## 5. 来源多样性（Source Diversity）：分桶并行（retriever.go:725-805）

```go
// DefaultRetrieverConfig —— 默认分桶
DiversityCandidates: map[constants.SourceType]int{
	constants.SourceTypeReport: 15,  // 研报 15 条
	constants.SourceTypeNews:   10,  // 新闻 10 条
	constants.SourceTypeKB:     10,  // 知识库 10 条
},
```

### 5.1 为什么需要

不限来源的检索常常被某一种来源"霸屏"（新闻量大、更新快，容易把研报和知识库挤没）。**分桶 = 每个来源各取各的候选数**，合并后再交给 Rerank 自然排序——**由相关性决定去留，而不是由数量决定去留**。

### 5.2 触发条件（`shouldApplyDiversity`）

- ✅ 启用且配置非空
- ❌ 用户**明确指定**了 source_type / source_id / file_id 时**不走分桶**（用户有明确来源意图，不走多样性策略）
- 各桶并行搜（errgroup）→ 按 ChunkID 去重合并 → 进下游 RRF + Rerank

> **一句话**：多样性是"无明确意图时的刻意平衡"；一旦用户说清要什么来源，立即让位于明确的过滤条件。

---

## 6. Deep 模式：NebulaGraph 多跳遍历（graph_traverser.go）

### 6.1 完整流程（每跳三步，最多 MaxHops 跳）

```
Step A  获取当前实体集合的全部邻居边（NebulaGraph，跳过已访问节点）
Step B  PG 补全边与邻居实体的描述（enrichEdgeCandidates，让 LLM 能读懂）
Step C  LLM 决策：这些候选里哪些与本问题相关？选它作为下一跳种子？
        （递归进行，直到：无更多边 / 到达 MaxHops / LLM 判定已足够）
```

- 入参：`seedEntityIDs`（来自 Fast 的 Route 2 实体搜索结果）+ 原始问题 + tenantID
- **LLM 在循环里当"裁判"**：每跳都要判断"该不该继续往里走"——这是**图遍历 + LLM 决策**的混合模式
- 结果：追加的 Entities/Relations 拼进 `RetrieveResult`，与 Fast 结果一起返回
- **失败兜底**：NebulaGraph 不可用/遍历失败 → `StopReason` 说明原因，**返回 Fast 结果不影响主链路**（retriever.go:395-400）

### 6.2 Fast vs Deep 一句话

> **Fast = 能搜到**（三路向量，快）；**Deep = 能推理**（在图里沿着关系链走，把"间接关联"揪出来）。Deep 是 Fast 的超级增强版，不改变主链路结构，只追加图遍历。

---

## 7. 与通用检索范式的对照

| 维度 | BondSeek | 通用朴素 RAG（简单向量检索） |
|------|----------|---------------------------|
| Query 增强 | 四步扩展（重写/抽象/分解/变体） | 无，原样检索 |
| 召回路数 | 三路并行（chunk/entity/relation）+ 证据映射 | 单路 chunk 向量 |
| 结果融合 | RRF 倒数排名融合（异构不可比分） | 无（单一排序） |
| 相关性精排 | 交叉编码器 Rerank（Top30） | 无 |
| 时间处理 | 分区裁剪 + 月内精筛 + 衰减（分来源） | 无 |
| 权限隔离 | DataScope + tenant/user 过滤表达式 | 弱或无 |
| 防 OOM | 分区 + 软分批 | 无 |
| 成本/延迟 | 高（多次 LLM 预处理 + Graph） | 低 |
| 适用 | 金融决策、多源异构、时效敏感 | 通用闲聊、小规模知识库 |

**一句话**：BondSeek 把检索做成了一条**"费 LLM 但不费结果"**的重管线——用 LLM 做查询理解和决策（高智商处）、用确定性过滤做安全和性能兜底（高确定性处），各自用在刀刃上。

---

## 反模式（本主题避免）

- **向量库存正文**——Milvus 只存向量+ID+过滤字段，正文永远 PG，检索回源取正文
- **三路结果直接相加**——量纲不同的路用 RRF（只看名次）融合，不拍脑袋加权
- **Rerank 失败就报错**——降级用融合原顺序，增强路径永远不拖垮主链路
- **关键词抽了不用**——`_ = keywords` 是埋点（预留 Hybrid Search），真上线要有对应产出
- **gauss 一刀切衰减**——研报 3 个月后仍有价值，exp 长尾比 gauss 断崖更适合金融
- **不看分区直接全扫描**——`publish_year_month in [...]` 裁剪是防 Milvus OOM 的第一道闸

---

## 📖 名词标注表（完整中英对照）

> 本文（RAG 检索系统）全部关键英文术语，看懂这张表 = 读懂本文。

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **RAG** (Retrieval-Augmented Generation) | 检索增强生成 | 先检索相关文档片段再回答，减少幻觉 |
| **Query** | 查询/问题 | 用户输入的一句话，检索管线的输入 |
| **ResolvedQuery** | 消解后的查询 | 把"它""最近"等指代和时间词替换成具体内容后的查询 |
| **Query Rewrite** | 查询重写 | 指代消解 + 时间规范化（"它"→"贵州茅台"） |
| **Step-Back** | 退一步提问 | 从具体问题生成更宏观的背景问题，提供语境 |
| **Decompose** | 问题分解 | 把复杂问题拆成多个子问题（≤5 个） |
| **Multi-Query** | 多查询重写 | 每个子问题生成多个同义问法（≤3 个，并行） |
| **Query Expansion** | 查询扩展 | Rewrite + Step-Back + Decompose + Multi-Query 的总称 |
| **关键词抽取 (Keyword Extract)** | 提取关键词 | 抽出高低级关键词（高级=概念级，低级=实体级） |
| **Embedding** | 向量化/嵌入 | 文本→数字向量，语义相近的数字相近 |
| **Chunk** | 分块/片段 | 文档切片，检索的最小单位 |
| **Entity** | 实体 | 知识图谱节点（公司、债券、行业） |
| **Relation** | 关系 | 知识图谱边（"控股""担保""发行"） |
| **TopK** | 前 K 条 | 取相似度最高前 K 个结果（chunk/entity/relation 各 20） |
| **候选池 (Candidate Pool)** | 候选集合 | Rerank 前的备选结果集合（多取，精排后截断） |
| **数据库过滤 (Filter)** | 筛选条件 | Milvus 标量字段过滤（权限/来源/时间） |
| **Expression** | 过滤表达式 | Milvus 的过滤语法（如 `data_scope == "public"`） |
| **RRF** (Reciprocal Rank Fusion) | 倒数排名融合 | 多路排序合并：Σ 1/(rank+k)，k=60 |
| **Rerank** | 重排序 | 用交叉编码器精排候选，取 Top30 |
| **交叉编码器 (Cross-Encoder)** | 拼接打分模型 | 把问题+片段拼一起让模型判断相关性 |
| **候选截断** | 精排后收窄 | 多召回 → 精排 → 只留最相关 N 条 |
| **DataScope** | 数据作用域 | public 公共 / private 私人 / all 混合 |
| **TenantID / UserID** | 租户 ID / 用户 ID | 多租户隔离的标识，拼进过滤表达式防越权 |
| **partition_key** | 分区键 | Milvus 按 `publish_year_month` 按月切分区 |
| **分区分批 (partition pruning)** | 分区裁剪 | 只搜相关月份分区，防全库扫描 OOM |
| **软分批 (soft batching)** | 分批查询 | 超 24 个月按月切批并发查再合并 |
| **Decay** | 时间衰减 | 越旧的新闻/研报权重越低（exp 指数衰减） |
| **exp 指数函数** | 长尾衰减 | 7 天满分 → 37 天 50% → 97 天 12.5% |
| **gauss 高斯函数** | 钟型衰减 | 67 天权重已 ~0.06（断崖，不适合研报） |
| **来源多样性 (Source Diversity)** | 分桶平衡 | report:15 / news:10 / kb:10 各取候选再合并 |
| **图谱遍历 (Graph Traverse)** | 图数据库多跳走查 | Deep 模式沿关系链多跳，LLM 每跳决策是否继续 |
| **种子实体 (Seed Entity)** | 起始节点 | Route 2 搜到的实体，作为图遍历的起点 |
| **邻居边 (Neighbors)** | 相邻关系 | 图里与某实体直接相连的边 |
| **Hop** | 跳数 | 图遍历走一步算一跳（MaxHops 限制深度） |
| **证据映射 (Evidence)** | 实体↔文本桥 | PG 里 entity→chunk 的映射，图谱结果反查回原文 |
| **补全 (Enrich)** | 填充描述 | PG 把实体/关系的规范名和描述填回结构体 |
| **errgroup** | Go 并发组 | 多路并行 + 出错的统一传播（Golang 并发原语） |
| **回调 (Callback)** | 进度通知 | 各阶段结果推给调用方（前端流式展示用） |
| **降级 (Fallback)** | 失败兜底 | 增强环节失败用基础结果，不阻塞主链路 |
| **埋点 (Code Hook)** | 预留接口 | `_ = keywords` 占位，为未来 Hybrid Search 预留 |
| **OOM** | 内存耗尽 | Out Of Memory，大批量查询的致命风险 |
| **Milvus / NebulaGraph / PostgreSQL** | 三个数据库（专有名词） | 向量库 / 图数据库 / 关系型主库 |

---

## 关联文档

- [`case-study-bondseek-memory.md`](./case-study-bondseek-memory.md) — 整体案例（Context 管理 + 三层记忆 + RAG 总览）
- [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) — L1 用户画像专项（注意：画像与图谱证据都经 `result.MemoryContext` 注入 RAG）
- [`../topics/memory-and-state/context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) — 理论框架（上下文/记忆区别）
- [`../topics/memory-and-state/session-compression-in-agent-systems.md`](../topics/memory-and-state/session-compression-in-agent-systems.md) — 压缩策略（检索上下文如何缩放进窗口）
- [`notes/agent-interview-questions.md`](../notes/agent-interview-questions.md) — 面试话题索引（第 8 题：RAG 检索流程）
