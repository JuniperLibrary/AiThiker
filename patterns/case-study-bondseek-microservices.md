---
title: BondSeek 微服务拆分（5 服务拆法 + 数据所有权 + EventBus + 网关认证）
tags: [case-study, microservices, architecture, service-split, eventbus, gateway, golang]
created: 2026-10-29
updated: 2026-10-29
status: draft
---

# BondSeek 微服务拆分（5 服务拆法 + 数据所有权 + EventBus + 网关认证）

> **案例对象:** [bondseek-ai](https://github.com/) — 债券行业多租户 AI SaaS 投研平台（Go 1.24 微服务）
> **分析范围:** 一个 AI SaaS 平台怎么拆微服务——按什么维度拆（业务边界/数据所有权/基础设施依赖）、5 个服务各干什么、跨服务的共享代码怎么管理（common）、服务间怎么通信（EventBus）、认证怎么统一（网关 ForwardAuth）
> **资料代码:** `go.work`（5 模块 workspace）+ `AGENTS.md`（架构表、表命名、EventBus 规范）+ `.cursor/rules/bondseek.mdc`（工程规范 v3.3）+ `README.md`（K3s 网关架构）
> **关联文档:** [多 Agent 编排](./case-study-bondseek-multi-agent.md)、[可观测性](./case-study-bondseek-observability.md)、[SSE 断线恢复](./case-study-bondseek-sse-recovery.md)
> **面试话题索引:** [agent-interview-questions.md](../notes/agent-interview-questions.md) — 第 13 题"微服务怎么拆"（本文即答案素材）

## 📌 名词速查表（先看这里）

> 本文讲微服务拆分，核心是"按什么拆、拆完怎么连、公共代码放哪"。先看这里（完整对照见文末"名词标注表"）：

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **Microservices** | "微服务" | 把一个大系统拆成多个独立部署的小服务 |
| **Module** | "模块" | Go 的一个独立代码包/编译单元（本项目一个服务一个） |
| **Service** | "服务" | 独立部署、独立端口、独立数据库表的进程 |
| **Go Workspace** | "Go 工作区" | go.work 把多个模块串起来共同开发调试 |
| **EventBus** | "事件总线（专有名词）" | 服务间异步广播消息的通道（NATS JetStream） |
| **Stream** | "流（专有名词）" | 事件总线上按主题划分的消息通道 |
| **Subject** | "主题" | 事件的名字（如 system.user.created） |
| **Gateway** | "网关" | 所有请求的第一道门：转发 + 统一认证 |
| **ForwardAuth** | "转发认证（专有名词）" | 网关把请求转发给认证服务验证，通过才放行 |
| **RBAC** | "基于角色的访问控制" | 按角色分配权限（用户→角色→权限） |
| **Common** | "公共层" | 所有服务共享的代码库（不是独立服务） |
| **DOM 边界 / 数据所有权** | "领域边界" | 数据归哪个服务管，其他服务不能直接碰 |

---

## 🎯 TL;DR 冲突摘要（先看这段）

> **这篇文档解决的核心矛盾：单体应用"好写好难养"——代码都在一个仓库一个进程里，改一处全系统回归、扩一个模块要扩整台机器、新手随便碰全局函数。** 微服务拆开的本质是**交换**：用"运维复杂度"（多服务部署/监控/通信）换"团队独立性与故障隔离"（A 服务挂了 B 还能跑、A 服务疯狂发布 B 不受影响）。但拆微服务最大的陷阱是**只拆代码不拆数据**——如果所有服务还共用一个数据库互相 JOIN，那等于换汤不换药，还多了一堆网络调用。BondSeek 的破局思路一句话总结：**"按'业务边界 + 数据所有权'拆——每个服务有自己前缀的表（t_system_/t_agent_/t_data_/t_sandbox_）、自己的 Redis 前缀、自己的基础设施依赖，服务间绝不直接查对方的表，只通过 EventBus 发事件拿通知；认证统一放网关，共享代码放 common，服务只跑自己的业务。"** 读完全文你只需要带走一个判断标准：**微服务拆得对不对，看一条数据在系统里有没有'唯一的爸爸'——有唯一归属才是真拆分，谁都能改就是伪拆分。**

---

## 1. 全景：5 服务拆法（先记住这张表）

BondSeek 是一个 Go workspace（go.work），5 个 module，其中 **4 个是独立服务 + 1 个是共享库**：

| Module | 角色 | 入口 | 端口 | 一句话职责 |
|--------|------|------|------|-----------|
| `bondseek-system` | 系统服务 | `cmd/server/main.go` | 8001 | 认证、用户、租户、RBAC（**管"人"**） |
| `bondseek-agent` | Agent 服务 | `cmd/server/main.go` | 8002 | LLM Agent、对话、上下文（**管"脑"**） |
| `bondseek-data` | 数据服务 | `cmd/server/main.go` | 8003 | 债券数据、行情同步、RAG 检索（**管"料"**） |
| `bondseek-sandbox` | 沙箱服务 | `cmd/server/main.go` | 8004 | 代码执行、策略回测（**管"手"**） |
| `bondseek-common` | 共享库 | —（不启动） | — | 模型、DTO、事件、基础设施客户端（**管"通用件"**） |

每个服务内部统一分层：`internal/{handler, service, repository, container}`。

> **面试金句：5 个模块是 4 服务 + 1 库，不是 5 个服务。** 很多人口头说"拆了 5 个微服务"，其实 common 是共享库。主动说清"几个服务几个库"，说明你真的拆过。

---

## 2. 拆分的三个维度（怎么决定"拆成什么样"）

BondSeek 的拆分决策可以总结成三个同时成立的维度：

| 维度 | BondSeek 怎么拆 | 拆错会怎样 |
|------|----------------|-----------|
| **① 业务边界** | system 管"人"，agent 管"脑"，data 管"料"，sandbox 管"手"——按业务职责一刀切 | 两个服务都在管用户，改需求要跨服务改 |
| **② 数据所有权** | 每个服务有自己的表前缀 + Redis 前缀，**绝不跨服务查表** | 共用一库互相 JOIN，拆了等于没拆 |
| **③ 基础设施依赖** | system 只连 PG+Redis+NATS；data 连 ClickHouse+MinIO+Milvus；agent 全连 | 服务依赖复杂度爆炸，"拆分"没带来"隔离" |

**核心思想：微服务拆的是"边界"不是"代码行数"。** 代码拆到哪个文件不重要，重要的是**数据归谁管、依赖归谁管、出问题影响谁**。

---

## 3. 数据所有权：拆分的真正命门 ⭐⭐⭐

### 3.1 表前缀 = 数据归属的"身份证"

```sql
t_system_users          -- system 服务（用户）
t_agent_conversations   -- agent 服务（对话）
t_data_bonds            -- data 服务（债券）
t_sandbox_executions    -- sandbox 服务（执行记录）
```

**规则**：表名必须带服务前缀，**服务 A 的代码绝不能写服务 B 前缀的表**——这是通过命名规范强行实施的"数据隔离"，比靠自觉靠谱得多。

### 3.2 Redis Key 前缀（同样逻辑）

```
system:user:{userID}:info
agent:conversation:{convID}:state
```

Redis 也按服务分前缀，缓存同样有归属。不这样做，两个服务可能撞 key 互相覆盖。

### 3.3 模型三向拆分（common 内部再按服务分）

共享库里的数据模型也保持边界：

```
bondseek-common/models/
├── db/{service}/     ← DB 实体（system/agent/data/sandbox 各一目录）
├── dto/              ← API 请求/响应结构
└── event/            ← EventBus 消息结构
```

**规则**：
- DB 实体 struct **必须**定义在 common 的 `models/db/{service}/` 下，各服务 import 引用
- Repository 层**只写 SQL**，从 common import struct
- **禁止**在 repository/service/handler 里自定义 DB 实体
- **禁止**跨服务直接引用对方的 repository 包

> **面试金句**：数据所有权是"用一条一条的命名规范钉死的"——表前缀、Redis 前缀、模型目录，三层都强制服务边界。**约定 + 强制 = 数据隔离真的落地。**

---

## 4. 每个服务介绍（拆完各自干什么）

### 4.1 bondseek-system（8001）——管"人"

- 认证授权：JWT 生成/验证，支持 Gateway ForwardAuth
- 用户/租户管理：CRUD、多租户隔离
- RBAC：角色权限、数据权限（global/personal）、权限树
- 审计日志：完整操作审计 + 多维度查询

**关键点**：它是唯一"能登录"的服务——其他服务不碰账号密码，通过网关注入的用户头拿身份。

### 4.2 bondseek-agent（8002）——管"脑"

- 多 Agent：债券分析/投资建议/数据处理专家
- 对话管理：会话创建、消息历史、上下文维护
- 上下文管理：压缩、向量检索、RAG 增强
- 流式响应：SSE
- 工具调用：数据查询、图谱查询、代码执行（**调 data/sandbox 的能力**）

**关键点**：它是"最复杂"的服务（LLM 调用、上下文、工具编排全在这），所以它连**所有**基础设施。

### 4.3 bondseek-data（8003）——管"料"

- 债券数据：基础信息、评级、发行人
- 行情同步：实时行情、历史 K 线（存 ClickHouse）
- 研报/政策：抓取、解析、向量化（存 Milvus）
- 知识图谱：实体关系（Apache AGE / 图数据库）
- 数据查询：债券、行情、图谱查询

**关键点**：它是"数据大本营"，被 agent 服务当"工具的底层数据源"调用。

### 4.4 bondseek-sandbox（8004）——管"手"

- 安全隔离：独立代码执行环境、资源限制
- Runner 管理：调度/管理代码执行环境
- 策略回测：量化策略回测、性能分析
- 执行日志 + 工作目录持久化

**关键点**：它是"会执行代码的地方"，AI 生成的代码到这里跑——**必须隔离**，所以单独拆一个服务（安全问题只影响一个小面）。

### 4.5 bondseek-common——共享库（不是服务）

- `models/`：db/dto/event 三向模型
- `constants/`：Redis key、事件类型、Agent 类型、LLM 配置 key、WebSocket 类型
- `middleware/`：auth 中间件辅助（从请求头取用户，禁止手解析）
- `infra/`：基础设施管理器（统一初始化/关闭所有客户端）
- `agenttools/`：工具接口抽象（Name/Definition/Execute + ToolResult）
- `eventbus/`：事件发布/订阅封装
- `llm/`：LLM 客户端（openai/anthropic 按 provider 切换）
- `utils/`：权限检查、权限编码生成

**面试点**：**common 是"防止三个服务各写一套"的减熵器。** 如果 Redis key 格式四个服务各写各的，接口调用全靠猜——common 的 constants/ + middleware/ 保证全系统一种写法。

---

## 5. 基础设施按需初始化（服务只连自己需要的）⭐⭐

每个服务启动时**只初始化自己依赖的客户端**：

```go
// bondseek-system: PostgreSQL + Redis + NATS
infraManager.InitPostgres()
infraManager.InitRedis()
infraManager.InitNATS()

// bondseek-agent: 全部组件（它最复杂）
infraManager.InitAll()

// bondseek-data: PostgreSQL + Redis + ClickHouse + MinIO
infraManager.InitPostgres()
infraManager.InitRedis()
infraManager.InitClickHouse()
infraManager.InitMinIO()
```

统一通过 `infra.NewManager(cfg)` → `defer Close()` 管理生命周期。

> **面试点**：这就是"依赖按需"——**服务不用的东西不连**。一个只查 PG 的服务跑去连 Milvus，等于把一个全系统的依赖炸弹又背回身上。**拆微服务的意义之一就是"故障面小 + 依赖面小"。**

---

## 6. 服务间通信：EventBus（NATS JetStream）⭐⭐

### 6.1 为什么不直接 HTTP 调

服务间业务通信用**事件（异步）**而非请求（同步）——解耦、可重试、不阻塞主链路。

### 6.2 Stream 划分（按服务分通道）

| Stream | Subject | 说明 |
|--------|---------|------|
| EVENTS | `events.>` | 通用事件流 |
| SYSTEM | `system.>` | 系统服务事件 |
| AGENT | `agent.>` | Agent 服务事件 |
| DATA | `data.>` | 数据服务事件 |
| SANDBOX | `sandbox.>` | 沙箱服务事件 |

### 6.3 事件命名规范

```
{service}.{entity}.{action}
例：system.user.created  /  agent.session.created  /  data.bond.updated
```

### 6.4 快慢双通道（FastLane / SlowLane）

| 通道 | 用途 | 特点 |
|------|------|------|
| **FastLane（快线）** | WebSocket 推送、实时通知、在线状态 | 内存队列、实时、**丢失可容忍** |
| **SlowLane（慢线）** | 数据同步、审计日志、重要业务事件 | 持久化、可靠、**不可丢失** |

> **面试金句**：事件总线不是"所有事件都可靠"——**按热度/丢失代价分快慢线**：实时推送走内存（丢了重新推），审计日志走持久化（丢了要命）。这跟后面 SSE 案例的"先写 replay 后发 live"是同一个思想：**重要数据永远有可靠通道兜底。**

---

## 7. 网关统一认证（Gateway ForwardAuth）⭐⭐

### 7.1 为什么必须统一

**4 个服务都要认证，但认证逻辑只该写一遍**——否则每个服务都得自己验 JWT、自己被攻击面。

### 7.2 流程

```
客户端请求 → K3s Gateway (Traefik)
           → 免鉴权路径（/login, /health）直接放行
           → 否则 ForwardAuth 转发到 bondseek-system /auth/verify
           → System 验证 JWT → 查用户角色/权限
           → 写响应头：X-User-ID / X-Tenant-ID / X-Roles / X-Permissions
           → 网关把这些头注入到原始请求头
           → 路由到目标服务
           → 服务用 middleware.MustGetCurrentUserID(c) 等读身份（不解析 JWT）
```

### 7.3 三个关键设计

| 设计 | 解决什么 |
|------|---------|
| **认证收敛在 system** | 其他服务不存密钥、不验 JWT——入侵面减小 |
| **服务读头不解析 Token** | 中间件封装 `MustGetCurrentUserID()`，禁止手解析 Header（规范强制） |
| **本地开发用前端中间件模拟** | 本地没有网关——前端 Dev Server 中间件替身履行 ForwardAuth，4 服务照样开发 |

> **面试点**：**"认证不做第二遍"**——网关查一次、注入头、全链路复用，这是微服务认证的标准做法（对应 k8s Gateway API 的 ForwardAuth 模式）。

---

## 8. 部署与扩展（拆分带来的好处兑现）

### 8.1 本地开发

```
# 4 个终端各跑一个服务（本地 Docker Compose 只起基础设施）
cd bondseek-system && go run ./cmd/server   # 8001
cd bondseek-agent   && go run ./cmd/server   # 8002
cd bondseek-data    && go run ./cmd/server   # 8003
cd bondseek-sandbox && go run ./cmd/server   # 8004
```

### 8.2 生产（K3s + Gateway API）

- 每个服务独立 Deployment + Service + ConfigMap/Secret
- **独立扩缩容**：agent 服务是 LLM 密集，可以 `replicas=3`；system 服务可能 1 个就够
- **独立回滚**：`kubectl rollout undo deployment/bondseek-agent`
- 配置：ConfigMap（非敏感）/ Secret（密钥）分层

> **面试金句：拆微服务的收益不是"代码整齐"，而是三个权力——独立部署、独立扩缩、独立回滚。** Agent 服务是 LLM 调用热点，单独多副本；system 服务瘦，一个副本扛得住——这就是拆分兑现的价值。

---

## 9. 核心设计哲学提炼（为什么这么拆）

| 哲学 | BondSeek 体现 | 类比 |
|------|--------------|------|
| **数据有唯一爸爸** | 表前缀 + Redis 前缀 + 模型目录，一个数据只有一个归属服务 | 公司里每块地有唯一负责人 |
| **认证只做一遍** | Gateway ForwardAuth 收敛到 system | 大楼只有一个大门保安 |
| **通信走事件不直连** | EventBus 快慢线，服务间不互相查表 | 部门间靠"公告栏"通知，不互相翻抽屉 |
| **公共代码收敛** | common 统一模型/常量/中间件/基础设施 | 公司统一 HR 系统，各部门不各搞一套 |
| **依赖按需** | 服务只初始化自己需要的客户端 | 岗位只配该岗位的工具 |

---

## 10. 一句话总结 + 30 秒面试框架

```
一句话：微服务拆的是"边界"不是"代码"——数据有唯一爸爸、认证只做一遍、
        通信走事件、公共放 common、依赖按需连。
```

### 面试回答框架（30 秒版）

> "我们的平台按**业务边界 + 数据所有权**拆成 4 服务 + 1 共享库：**system** 管认证/用户/RBAC，**agent** 管 LLM 对话与工具编排，**data** 管债券数据/行情/RAG，**sandbox** 管代码执行与回测，**common** 只做共享库。拆分的命门是**数据所有权**——每个服务有自己的表前缀（t_system_/t_agent_/t_data_/t_sandbox_）和 Redis 前缀，服务间绝不跨库查表；跨服务通信走 **EventBus**（NATS），按服务分 Stream、事件命名 `service.entity.action`，并按丢失代价分**快慢线**（实时推送走 FastLane 内存，审计/数据同步走 SlowLane 持久化）；认证统一放**网关 ForwardAuth** 收敛到 system，其他服务只从请求头拿身份；共享代码（模型/DTO/中间件/基础设施客户端）集中在 common，服务只按需初始化自己的依赖。这样每个服务能**独立部署、独立扩缩容、独立回滚**——agent 是 LLM 热点多副本，system 瘦一个副本就够。"

---

## 反模式（本主题避免）

- **只拆代码不拆数据**——都共用一个库互相 JOIN，等于换汤不换药（还多点网络开销）
- **服务 A 直接查服务 B 的表**——破环数据所有权，故障和耦合一起回来
- **每个服务自己写认证**——攻击面翻 4 倍，JWT 密钥到处放
- **服务间同步 HTTP 调用耦合**——A 挂了 B 全挂，应该用事件解耦
- **公共代码到处复制**——Redis key 格式各写各的，跨服务调试全靠猜
- **服务连接所有基础设施**——一个查 PG 的服务跑去连 Milvus，依赖炸弹背回身上

---

## 📖 名词标注表（完整中英对照）

> 本文（微服务拆分）全部关键英文术语，看懂这张表 = 读懂本文。

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **Microservices** | 微服务 | 拆成多个独立部署/独立数据库的服务的架构风格 |
| **Service** | 服务 | 独立端口、独立表、独立部署的进程 |
| **Module** | 模块 | Go workspace 里的独立编译单元（一服务一模块） |
| **Go Workspace** | Go 工作区 | go.work 把多模块串起本地共同开发 |
| **Monolith** | 单体应用 | 所有功能在一个进程里的架构 |
| **EventBus** | 事件总线 | 服务间异步发布/订阅消息的通道（NATS） |
| **Stream** | 流 | 事件按主题划分的通道（EVENTS/SYSTEM/AGENT/DATA/SANDBOX） |
| **Subject** | 主题 | 事件的精确名字（system.user.created） |
| **FastLane / SlowLane** | 快线 / 慢线 | 内存实时通道 / 持久化可靠通道 |
| **Gateway** | 网关 | 请求入口：转发 + 认证（K3s Gateway API / Traefik） |
| **ForwardAuth** | 转发认证 | 网关转发请求给认证服务，验证通过才放行 |
| **RBAC** | 基于角色的访问控制 | 权限挂在角色上、角色挂在用户上 |
| **JWT** | JSON Web Token | 登录后签发的身份凭证（15 分钟过期） |
| **Common** | 公共共享库 | 非服务，所有服务共享的代码包 |
| **DTO** | 数据传输对象 | 前后端传输用的结构（不带 DB 字段） |
| **DB Entity** | 数据库实体 | 对应一张表的 struct（models/db/{service}/） |
| **Data Ownership** | 数据所有权 | 每条数据的唯一归属服务（表前缀体现） |
| **契约 (Contract)** | 接口约定 | 服务间共享的数据格式（DTO/event 结构） |
| **Deployment** | 部署单元 | K8s 里最小独立部署单位 |
| **Replica** | 副本数 | 一个服务跑几个实例（扩缩容） |
| **Rollback** | 回滚 | 出问题退回上一版本 |

---

## 关联文档

- [`case-study-bondseek-multi-agent.md`](./case-study-bondseek-multi-agent.md) — 编排"脑"里具体怎么协作（工具调用树 / recordEvent 事件）
- [`case-study-bondseek-observability.md`](./case-study-bondseek-observability.md) — 审计日志 + 调用树（微服务的横切观测如何在多服务间串起来）
- [`case-study-bondseek-sse-recovery.md`](./case-study-bondseek-sse-recovery.md) — SSE 断线恢复（agent 服务的流式能力 + 可靠性）
- [`case-study-bondseek-rag.md`](./case-study-bondseek-rag.md) — RAG 检索（data 服务承担"料"，agent 调它）
- [`notes/agent-interview-questions.md`](../notes/agent-interview-questions.md) — 面试话题索引（第 13 题）
