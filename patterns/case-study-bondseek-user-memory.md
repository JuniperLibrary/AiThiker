---
title: BondSeek 用户记忆（L1 用户画像）专项分析
tags: [case-study, user-memory, user-profile, memory-write-path, explicit-inferred, golang]
created: 2026-10-20
updated: 2026-10-20
status: draft
---

# BondSeek 用户记忆（L1 用户画像）专项分析

> **案例对象:** [bondseek-ai](https://github.com/) — 债券行业多租户 AI SaaS 投研平台（Go 1.24 微服务）
> **分析范围:** L1 用户画像的完整实现：数据模型 / 提取管线 / dedup 博弈 / 合并 / 遗忘 / 召回注入 / API 面 / 触发时机
> **资料代码:** `bondseek-common/memory/`（profile/、worker/、filter/、common/）+ `bondseek-agent/internal/handler/memory_handler.go` + `deployments/sql/postgresql/0001_schema.sql`（2026-10-20 代码级深挖）
> **关联文档:** [整体案例分析](./case-study-bondseek-memory.md)、[Context 与 Memory 本体论](../topics/memory-and-state/context-vs-memory.md)

## 📌 名词速查表（先看这里）

> 本文深入用户画像实现，术语多且密。核心词先看这里（完整对照见文末"名词标注表"）：

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **L1** | "用户画像层" | 记住"用户是谁"——偏好、风格、关注领域 |
| **tag / category** | "标签 / 类别" | 画像的分类维度（tag 是英文固定枚举） |
| **explicit** | "明说的" | 用户亲口说的话（可信度最高） |
| **inferred** | "推断的" | 系统从用户行为推测的（需多次证据） |
| **confidence** | "置信度" | 这条画像"有多可信"（0~1，≥0.85 才入库） |
| **dedup** | "去重" | 新记忆和旧记忆撞车时如何裁决（增/改/删/不动） |
| **ingest** | "摄取/写入" | 把对话内容加工成记忆的过程 |
| **cron** | "定时器" | 定时任务（如每月清理过时记忆） |

---

## 🎯 TL;DR 冲突摘要（先看这段）

> **这篇文档解决的核心矛盾：系统要"记住用户"，但记忆会错、会脏、会过时、会互相打架——比"没有记忆"更糟的是"记住了一个错的你"。** 用户说一句"我只买 AAA"就被永远锁死？不行。系统自己猜的偏好能当用户亲口说的？更不行。BondSeek 的破局思路一句话总结：**"给每条画像标'可信度级'（explicit 明说 > inferred 推断），再用四道质量闸 + 博弈规则强制分层——明说的永远压过推断的，推断的自相矛盾时保守取低。"** 读完全文你只需要带走一个画面：**用户记忆不是"记下来就完事"，而是一场持续的"可信度博弈"——谁说的（explicit/inferred）、多可信（confidence）、会不会过期（stale）、撞车怎么裁决（dedup）——每个问题都有软件规则在把关，而不是靠 LLM 的自觉。**

---

## 1. 核心结构：`category → tag → feature_name → value` 四级

用户记忆不是自由文本，而是**封闭枚举维度**（枚举=只有固定几个选项可选）的**结构化画像**（画像=对用户的描述档案）（设计文档 §3.1、§3.2）：

| tag（英文枚举） | 中文维度 | 允许 source_type（来源类型） | 生效门槛 |
|---|---|---|---|
| `investment_preference` | 投资偏好 | explicit 或 inferred | explicit 1 次；inferred 需 ≥3 次同向证据 |
| `interaction_style` | 交互风格 | **仅 explicit** | 1 次（严禁从 assistant 输出风格推断——"LLM 照镜子"禁令，即模型自问自答不能当用户偏好） |
| `domain_focus` | 领域聚焦 | **仅 inferred** | ≥3 次用户主动提问（看提问集中度，不看回答内容） |

### explicit vs inferred：可信度分层

| 维度 | explicit（明说） | inferred（推断） |
|---|---|---|
| 来源 | 用户直接陈述："我只买 AAA""不要日报" | 跨多轮行为聚合：反复提问/选择 |
| 置信度 | 1.0，1 次即生效 | 0.85 起，≥3 次同向才生效 |
| 证据 | 不需要 | value 内嵌 `(证据: 第3轮、第7轮、第9轮)` |
| 展示 | 直接 | 库内保留证据，对外展示/注入时 `StripEvidence`（剥离证据） |
| 优先级 | **永远压过 inferred**（软规则见 §4） | 无法推翻 explicit |

> **设计意图**：电商/内容推荐的画像可以模糊，金融事实画像必须可审计、可信度可分层。explicit 是用户亲口承诺，inferred 是系统推测——两者在 dedup 中的博弈规则就是这套分层的外化。

---

## 2. 数据模型与 DDL（t_agent_user_profiles）

`0001_schema.sql:697-718`，实体（数据对象）`models/db/agent/user_profile.go`：

```sql
CREATE TABLE IF NOT EXISTS t_agent_user_profiles (
    id                       BIGSERIAL,
    memory_id                UUID NOT NULL,          -- uuid v7，特征主标识（每条画像的唯一身份证号）
    tenant_id                UUID NOT NULL,
    user_id                  UUID NOT NULL,
    category                 VARCHAR(50) NOT NULL,   -- 中文大类：投资偏好/交互偏好/领域聚焦
    tag                      VARCHAR(100) NOT NULL,  -- 英文封闭枚举（固定三个）
    feature_name             VARCHAR(200) NOT NULL,  -- 特征键（LLM 生成，如"评级偏好"）
    value                    TEXT NOT NULL,          -- 特征值（inferred 内嵌证据，入库/召回时剥离）
    confidence               DOUBLE PRECISION DEFAULT 1.0,
    source_type              VARCHAR(20) NOT NULL DEFAULT 'inferred',  -- manual / explicit / inferred
    citation_ids             TEXT[],                 -- ADD 时填来源 conversation_id（这句记忆来自哪次对话）
    is_stale                 BOOLEAN DEFAULT false,
    is_deleted               BOOLEAN DEFAULT false,  -- 软删除（删了但留底）
    created_at / updated_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE UNIQUE INDEX uq_user_profiles_identity ON t_agent_user_profiles
    (tenant_id, user_id, category, tag, feature_name) WHERE (is_deleted = false);
```

**内容级幂等的基石**：业务唯一键 `(tenant, user, category, tag, feature_name) AND is_deleted=false`。`Insert`（store.go:62-131）先 UPDATE 命中唯一键再 INSERT（`dbx.ErrNoRows` 才插入）——Worker 崩溃重放 / dedup 误判 ADD 都不会产生重复行；`Insert` 返回 `inserted=false` 时调用方把事件从 ADD 改写为 UPDATE。

**配套审计表**：
- `t_agent_memory_history`（:682）— L1 变更审计：`event ∈ {ADD,UPDATE,DELETE,MERGE}`、`triggered_by ∈ {sleeptime_worker, user_explicit, consolidation}`、`old/new_content`
- `t_agent_memory_drops`（:665）— 四道闸丢弃审计：`drop_reason ∈ {short_conv, threshold:*, blacklist:*, value_score:*}`

---

## 3. 提取管线：四道质量闸 + 价值评分（异步）

### 3.1 编排（worker/sleeptime.go `handleIngestEvent`）

```
Step 0  用户开关 auto_memory_enabled（自动记忆开关）？关 → 整链路丢弃
Step 1  加载该会话 memory_extracted=false（未抽取标记）的分段（≤maxSegmentsPerRun 防爆）
Step 2  短对话(<100字符) → 记 drop（丢弃原因）+ 立刻标记 extracted（防 Scheduler 死循环）
Step 3  extractAndFilterL1 → 阈值/证据/黑名单三闸（闸=过滤关卡）
Step 4  extractAndFilterL2L3
Step 5  runValueScoring（L1+L2+L3 合并打分，总分≥8）
        └─ 评分失败 → 不 commit 不标记 → Scheduler 补偿重试（绝不放行污染）
Step 6  errgroup（并行工作组）并行 commit（L1→PG ∥ L2/L3→sandbox .md）
Step 7  全部成功 → 标记 memory_extracted=true
```

### 3.2 L1 三道闸（代码 enforce，非 LLM 自觉）

1. **阈值闸**（门槛：够可信才收）：`conf < 0.85` → drop `threshold:confidence<0.85`
2. **证据闸**：`inferred` 无 `(证据: …)` 形式标记（`filter.HasEvidence` 正则）→ drop `threshold:inferred_no_evidence`（只校验形式，不校验内容真实性）
3. **黑名单闸**：7 条正则（仪表盘/图表名、绝对日期、系统任务如早晚报、UUID 片段、文件扩展名、infra 关键字、问候语前缀）→ drop `blacklist:<tag>`

### 3.3 价值评分（filter/value_scorer.go）

- 常量：`scoreThreshold=8`（及格分）、`scoreTimeout=90s`、`scoreBatchSize=30`
- 四维 0-10：跨会话价值(0-3) / 长期有效性(0-3) / 信号源头(0-2) / 去会话独立性(0-2)
- L1 特判：`interaction_style + inferred → 直接 keep:false`（照镜子陷阱）；主语非用户本人 → 跨会话价值 0
- LLM 配置按职能拆 5 个 key：`memory_extract / consolidate / score / recall / write`（service.go:242-269）

---

## 4. dedup 博弈规则（common/dedup.go + prompts/dedup.go）

> **dedup = 去重**。这条新的画像候选和旧画像"撞车"时怎么办的裁决规则。

`DedupService.Decide` 返回 `DedupDecision{Event ∈ ADD/UPDATE/DELETE/NONE}`（增/改/删/不动四种裁决），执行前用 `buildIDMap`（整数 idx → memory_id，防 LLM 幻觉 UUID）。

**source_type 交互规则**（prompts/dedup.go:20-32）——本系统最值得借鉴的机制：

| 场景 | 决策 |
|---|---|
| 含义相同：新 explicit + 旧 inferred | **UPDATE 升级为 explicit**（用户亲口说了） |
| 含义相同：新 inferred + 旧 explicit | **NONE**（推测不覆盖明说） |
| 含义互斥：新 explicit + 旧 explicit | UPDATE 为新（用户明确改主意） |
| 含义互斥：新 explicit + 旧 inferred | UPDATE（explicit 覆盖 inferred） |
| 含义互斥：新 inferred + 旧 explicit | **NONE**（不能用推测推翻用户亲口说的） |
| 含义互斥：新 inferred + 旧 inferred | UPDATE，confidence = **min(新, 旧)**（矛盾时低置信度胜出） |

> 一句话：**explicit 永久压过 inferred；inferred 自相矛盾时置信度保守取低。** 这是一套软件实现的"用户陈述可信度分层"。

---

## 5. 合并与遗忘

### 合并（profile/consolidator.go）

- 触发：单 category/tag 条目 ≥ **15** 条（`ConsolidationThreshold=15`）→ LLM 合并；合并失败只记日志不阻塞本批 commit（二级优化）
- 规则：**只合并相同 source_type**；explicit+inferred 同义 → 删 inferred；互斥/矛盾不合并（留给 dedup 裁决）
- confidence：按信息量加权平均，**不高于最高值**
- 入参全程用整数 id（`profilesToNumberedJSON`）防 UUID 幻觉；写审计 `event=MERGE triggered_by=consolidation`

### 遗忘（worker/scheduler.go:161-178）

```sql
UPDATE t_agent_user_profiles
SET is_stale = true
WHERE is_deleted = false AND is_stale = false
  AND source_type = 'inferred'        -- 只标记推断特征，explicit/manual 不动
  AND confidence < 0.7
  AND updated_at < NOW() - INTERVAL '90 days'
```

- 入口：`MemoryService.RunMonthlyStaleCheck`（service.go:238）；**需外部 cron（定时器）触发，容器内未自动调度**（已知缺口）
- 召回时 stale（过时的）排最后（非 stale 优先 → confidence 降序）

---

## 6. 召回注入：XML 进 system prompt，非向量检索

```xml
<user_profile>
  <投资偏好>
    评级偏好: 只看 AAA 以上信用债
    久期偏好: 3 年以内 [stale]
  </投资偏好>
  <交互偏好>
    回复风格: 简洁
  </交互偏好>
  <领域聚焦>
    关注主题: 利率债期限策略
  </领域聚焦>
</user_profile>
```

- 生成：`recall.go:99-122 formatProfileXML`（category 分组、value 剥离证据、stale 打标记）
- **全量注入 system prompt**（画像总量小，不做向量筛选），经 `MemoryService.RecallProfileXML`（`{EnableProfile:true, MaxTotalTokens:2000}`）
- 同时注入 `memoryInstructions`（agent_def.go:740-746）：提醒 Agent "记忆可能过时，关键信息先验证 / 与用户当前表述矛盾时以当前为准 / 不要直接说'我的记忆显示'，自然地使用"
- **触发治理**：仅 `trigger=user` 且非子 Agent 时召回/写入；cron/context_init 一律跳过
- 注入路径：DefaultExecute（CodeCopilot/Quant Lab/Backtest/Export 共用，agent_def.go:348）+ AI Studio orchestrator（:748）+ RAG（rag/service.go:571，`result.MemoryContext`）

---

## 7. API 面（用户自助管理）

15 用户端点 + 3 超管端点（`/api/agent/memory/*`、`/api/agent/admin/memory/*`）：

| 方法 | 路径 | 语义 |
|---|---|---|
| GET | `/profile/list /profile?memory_id= /profile/history?memory_id=` | 列表/详情/变更审计 |
| POST | `/profile/create` | `source_type=manual`、confidence=1.0、**跳过四道闸**；feature_name 空则自动生成 `manual_<uuid8>` |
| POST | `/profile/update` | 只改 value（confidence 重置 1.0、source_type 重置 manual）；**所有权校验** `old.UserID != userID → ErrMemoryPermissionDenied` |
| POST | `/profile/delete` | 软删除 + 审计 DELETE |
| GET | `/settings + POST /settings/update` | `auto_memory_enabled` 用户开关 |
| GET | `/drops?layer=&page=&size=` | 丢弃审计查询 |
| POST | `/_debug/trigger-ingest` | e2e 调试：手动发布 ingest 事件 |

管理后台（IsSuperAdmin 门控）：`/admin/memory/profile`（多条件筛选）、`/admin/memory/episodic`、`/admin/memory/knowledge`（L3 全局）。

---

## 8. 触发与保存时机（5 条路径）

### 主触发：每次用户对话结束

| 触发点 | 位置 | 说明 |
|---|---|---|
| DefaultExecute step 16.5 | `agent_def.go:547-571` | 对话主循环**完全结束**（最终上下文已保存、done 事件前）→ **go goroutine 异步** `PublishIngestEvent`（30s timeout，不阻塞回复） |
| AI Studio | `orchestrator.go:287` | 编排器执行结束 → 异步 publish |
| RAG | `rag/service.go:592` | **流式生成结束**后异步 publish |

三重闸：`MemoryService 非 nil` + `非子 Agent` + `trigger=user`（cron/context_init 跳过）。

### 消费：NATS 事件实时处理（非定时批）

`SleeptimeWorker` 订阅 `agent.memory.ingest_requested`（SingleReplica），事件到达即走 §3 管线；保存动作（commitL1）发生：`ADD→写库 / UPDATE→覆写 / DELETE→软删`，每步写 `t_agent_memory_history`。

### 兜底：Scheduler 每 10 分钟补偿重投

- `time.NewTicker(10min)` 扫描 `memory_extracted=false` 且 `updated_at > 10分钟前` 的分段 → 重发 NATS
- 重试超 **5 次**（maxCompensateAttempts）→ 强制标记 extracted 放弃（防坏段霸占队列）
- **防护重点**：评分/commit 失败 → 有意**不标记**让 Scheduler 重试（重试本身幂等安全）

### 即时：用户手动 CRUD

`/profile/create|update|delete` 实时写库，跳过四道闸但写审计（`triggered_by=user_explicit`）——用户亲手动写的最高优先。

### 持续：段级标志复位（隐藏的再触发机制）

```sql
-- context/storage.go appendToSegment（同一分段追加新消息时）:
UPDATE t_agent_conversation_history
SET history = history || $1::jsonb,
    memory_extracted = false,   ← 新消息 → 段被重置为"可再抽"
    updated_at = NOW()
```

**含义**：同一会话下一轮对话产生新消息 → 已抽分段复位 → 下次 ingest 带着**累积上下文**再抽一轮 → 画像持续进化，直到被 stale 或合并收敛。

---

## 9. 触发时序总览

```
用户第 N 轮对话
  │ ← 触发1：Agent 执行完毕（DefaultExecute 16.5 / Orchestrator :287 / RAG :592）
  ▼
PublishIngestEvent(NATS) ──实时消费──▶ SleeptimeWorker 管线
                            │  开关→分段→抽取→三闸→价值评分→dedup
                            │  评分/commit 失败→不标记→Scheduler 每10分钟补偿(≤5次)
                            ▼
              t_agent_user_profiles（ADD/UPDATE/DELETE）
              + t_agent_memory_history 审计 + t_agent_memory_drops 审计
  │
  ▼
用户第 N+1 轮新消息 → appendToSegment 复位 memory_extracted=false
  │ ← 可再抽 → 画像持续进化
  ▼
月度 stale 检查（inferred+conf<0.7+90天 → is_stale，需外部 cron）
```

---

## 10. 与通用范式的对照

| 维度 | BondSeek L1 | mem0/LangMem 式 |
|---|---|---|
| 存储 | PG 结构化行（封闭枚举维度） | 向量库 + 非结构化文本 |
| 召回 | **全量 XML 注入 system prompt** | 向量相似度检索 |
| 写入 | 四道业务规则闸 + explicit/inferred 置信分层 | LLM 抽取 + 阈值 |
| dedup | explicit 压过 inferred 的博弈规则 + DB 唯一键幂等 | 相似度去重 |
| 审计 | 完整变更审计 + 丢弃审计 + 证据内嵌 | 弱审计 |
| 遗忘 | 软标记 stale（90 天）+ tag≥15 合并 | 时间衰减/删除 |
| 信任模型 | "LLM 不可信 → 业务规则层层兜底" | "LLM 可信 → 检索表达力优先" |

**一句话**：BondSeek 用"确定性业务规则"驯服 LLM 的不确定性——封闭枚举防跑题、证据内嵌保可审计、explicit/inferred 分层控可信度、整批放弃防污染。代价是每轮最多 6-7 次 LLM 调用（召回1+提取2+评分1+dedup2+合并0-1）。

---

## 反模式（本主题避免）

- **把画像当向量存**——金融画像必须可读、可编辑、可审计（PG 行 + Markdown 是刻意的）
- **inferred 覆盖 explicit**——推测永远不能推翻用户亲口说的
- **评分失败放行**——价值评分 LLM 失败必须整批放弃等补偿，绝不放行污染记忆库
- **无遗忘的画像膨胀**——tag ≥15 合并 + 90 天 stale 双机制缺一不可

---

## 📖 名词标注表（完整中英对照）

> 本文（用户记忆专项）全部关键英文术语，看懂这张表 = 读懂本文。

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **L1 (Level 1)** | 第一层（用户画像） | 记忆三层中的第一层：记住"用户是谁" |
| **User Profile** | 用户画像 | 对用户的描述档案：偏好、风格、关注领域 |
| **category** | 类别 | 画像的一级分类（中文）：投资偏好/交互偏好/领域聚焦 |
| **tag** | 标签 | 画像的二级分类（英文固定枚举）：preference/style/focus |
| **feature_name** | 特征名 | 画像的三级：具体特征键（如"评级偏好"） |
| **value** | 特征值 | 画像的四级：具体内容（如"只看 AAA"） |
| **枚举 (enum)** | 固定选项集 | 只能从固定几个值里选，防止乱写 |
| **source_type** | 来源类型 | 这条画像怎么来的：manual 手动/explicit 明说/inferred 推断 |
| **explicit** | 明说的 | 用户亲口直接说的话，可信度最高 |
| **inferred** | 推断的 | 系统从用户行为推测的结论，需多次证据 |
| **confidence** | 置信度 | 可信程度 0~1，≥0.85 才入库 |
| **evidence** | 证据 | inferred 画像内嵌的轮次依据 `(证据: 第N轮…)` |
| **citation_ids** | 引用来源 | 指向产生这条记忆的那次对话 ID |
| **strip (StripEvidence)** | 剥离证据 | 对外展示/注入时把证据标注去掉 |
| **stale** | 过时的 | 长期未更新被标记为"不可信" |
| **软删除 (soft delete)** | 假删除 | 标记 is_deleted=true 而非物理删行，可恢复可审计 |
| **UUID** | 通用唯一标识符 | 全局唯一的 ID（类似身份证号） |
| **BIGSERIAL** | 自增整数 | 数据库自动递增的整数主键 |
| **DDL** (Data Definition Language) | 建表语句 | 定义数据库表结构的 SQL |
| **UPSERT** | 更新或插入 | 有则更新、无则插入，防重复 |
| **幂等 (idempotent)** | 重复执行无害 | 做了两遍和做一遍结果一样（成功防重的基本功） |
| **ingest** | 摄取/写入 | 把对话内容加工成记忆存入系统的过程 |
| **管线 (pipeline)** | 流水线 | 数据按固定步骤依次处理的流程 |
| **黑名单 (blacklist)** | 禁用名单 | 7 条正则规则，命中的内容直接丢弃 |
| **正则表达式 (regex)** | 模式匹配语法 | 用规则描述"长什么样的文本"，计算机自动匹配 |
| **阈值 (threshold)** | 门槛/及格线 | 低于/高于就不通过的数值标准（如 conf<0.85 丢弃） |
| **评分 (value scoring)** | 价值打分 | LLM 给候选记忆打分，不及格（<8）不存 |
| **dedup** | 去重 | 判断新旧记忆是否重复/冲突，裁决增改删动 |
| **博弈规则** | 裁决逻辑 | 不同来源类型冲突时的处理优先级 |
| **ADD / UPDATE / DELETE / NONE** | 增 / 改 / 删 / 不动 | dedup 的四种裁决结果 |
| **merge / consolidation** | 合并/巩固 | 条目太多时把多条合成一条概括性记忆 |
| **consolidator** | 合并器 | 负责执行合并的模块 |
| **召回 (recall)** | 取回记忆 | 对话前把相关记忆取出来注入上下文 |
| **XML** | 可扩展标记语言 | 一种结构化文本格式（本系统用 `<user_profile>` 装画像） |
| **system prompt** | 系统提示词 | 设定角色与规则的固定指令（L1 画像注入这里） |
| **trigger** | 触发条件 | 什么情况下执行记忆（仅 user 对话、cron 跳过） |
| **cron** | 定时任务 | 定时执行的任务（如每月 stale 检查） |
| **Scheduler** | 调度器 | 定时/补偿任务管理器（每 10 分钟补偿重投） |
| **补偿重投 (compensate)** | 失败重试 | 抽取失败的分段定时重新尝试（≤5 次） |
| **segment** | 分段 | 对话历史按 500 事件切成的一段存储单元 |
| **memory_extracted** | 已抽取标志 | 该分段是否已被抽取过记忆的布尔标志 |
| **CRUD** | 增删改查 | Create 增 / Read 查 / Update 改 / Delete 删 |
| **HTTP API** | 网络接口 | 前端/外部调用后端功能的路径（如 /profile/create） |
| **IsSuperAdmin** | 超级管理员校验 | 只有管理员能访问的接口门控 |
| **NATS** | 消息队列（专有名词） | 服务间传递事件的通信组件 |
| **SleeptimeWorker** | 睡眠时间工人（专有名词） | 对话结束后在"空档期"异步抽取记忆的工人 |
| **Pg / PostgreSQL** | 关系型数据库（专有名词） | 主数据库，存画像表 |
| **sandbox** | 沙箱 | 隔离执行环境，本项目存 L2/L3 记忆文件 |
| **RPC** | 远程过程调用 | 程序调用另一台机器上的功能 |
| **LLM 照镜子** | 模型自证自说 | 从 AI 自己的输出风格推断用户偏好（错误做法） |

---

## 关联文档

- [`case-study-bondseek-memory.md`](./case-study-bondseek-memory.md) — 整体案例（Context 管理 + 三层记忆 + RAG）
- [`../topics/memory-and-state/context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) — 理论框架
- [`../topics/memory-and-state/session-compression-in-agent-systems.md`](../topics/memory-and-state/session-compression-in-agent-systems.md) — 压缩策略理论
