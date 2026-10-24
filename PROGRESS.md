# 渠道「模拟客户端」功能 — 高保真进度记录

> 最后更新：2026-10-18 10:52 UTC
> 工作目录：`/workspace/axonhub-local`（原 `/workspace/axonhub`，已迁移）
> Git：分支 `unstable`，基线提交 `a946fca4`（2026-10-16）
> 规划文件：`.agent/plans/client-simulation/{task_plan,findings,progress}.md`

---

## 1. 需求（用户原话）

> 「我想给项目增加模拟客户端的选项,在渠道的 操作列这里,新增一个模拟客户端的选项,点击后能弹出客户端选择dialog,然后参考cli-proxy-api这个项目,对应的实现 codex,claude客户端（CLI）的选项,另外还要实现opencode客户端（CLI）的模拟选项,然后对应的需要改造真正的发接口时,要真正的实现了对应的客户端模拟,我的目的是,在某些API限制了必须是特定CLI才能请求API,那么我需要去模拟这个CLI去请求API」

目标：在渠道维度选择一种 CLI 指纹，AxonHub 真实向上游发请求时伪装成该 CLI，用于绕过「仅允许特定 CLI 访问」的上游限制。

---

## 2. 已确认的设计决策

| # | 决策 | 说明 |
|---|------|------|
| 1 | 模拟深度 | 只改写 **User-Agent、CLI 专属 headers、query 参数**；不修改请求 body 语义 |
| 2 | 渠道类型联动 | 与渠道类型**完全独立**；任意渠道操作列都有入口；配置叠加在现有 transformer 之上；未配置时保持原行为 |
| 3 | 版本号 | Dialog 中可配置；默认 codex `0.159.0`、claude-cli `2.1.170`、opencode `1.18.31` |
| 4 | 协议边界 | 不改变渠道 API 格式与 endpoint 路径；Dialog 内仅给出协议匹配提示 |

协议提示：
- Codex → 需配合 OpenAI **Responses** 协议（建议 codex 类型渠道）
- Claude Code → 需配合 **Anthropic Messages** 协议（建议 claudecode 类型渠道）
- OpenCode → 使用 **Chat Completions**，可叠加在大多数 OpenAI 兼容渠道

---

## 3. 实现清单（逐文件）

### 3.1 后端 — 配置模型与持久化

**`internal/objects/channel.go`**
- 新增常量：`ClientSimulationTypeNone/Codex/ClaudeCode/OpenCode`（`"none"` / `"codex"` / `"claude-code"` / `"opencode"`）
- 新增 `ClientSimulationSettings{ Type, Version string }`（json: `type` / `version,omitempty`）
- 新增 `DefaultClientSimulationVersion(clientType)`：返回内置默认版本
- 新增 `NormalizeClientSimulation(*ClientSimulationSettings) (*ClientSimulationSettings, error)`：
  - `nil` / 空 / `none` → 返回 `nil`（关闭模拟）
  - 未知类型 → 报错
  - 空版本 → 填默认版本
  - 版本含空白/控制字符 → 报错
- `ChannelSettings` 末尾新增 `ClientSimulation *ClientSimulationSettings`（json: `clientSimulation,omitempty`）
- 存储层是 `field.JSON("settings", ...)`，**无需数据库迁移**

**`internal/server/biz/channel.go`**
- `createChannel` 与 `UpdateChannel` 的 settings 校验块中调用 `NormalizeClientSimulation`
- 校验失败返回 `invalid client simulation: ...`
- 归一化结果写回 `input.Settings.ClientSimulation`

### 3.2 后端 — GraphQL

**`internal/server/gql/axonhub.graphql`**
```graphql
type ClientSimulationSettings {
  type: String
  version: String
}

input ClientSimulationSettingsInput {
  type: String
  version: String
}
```
- `type ChannelSettings` 与 `input ChannelSettingsInput` 各新增 `clientSimulation`

**`internal/server/gql/gqlgen.yml`**
- `ClientSimulationSettings` / `ClientSimulationSettingsInput` 均映射到
  `github.com/looplj/axonhub/internal/objects.ClientSimulationSettings`

**生成物**
- `internal/server/gql/generated.go` 已重新生成，包含：
  - `ClientSettings.clientSimulation` 字段 resolver（`_ChannelSettings_clientSimulation`）
  - `unmarshalInputClientSimulationSettingsInput`
  - `ChannelSettingsInput.clientSimulation` 解析
- `internal/server/gql/models_gen.go` 内容与基线一致（新类型直接映射到 Go struct，未生成新 model）

### 3.3 后端 — 指纹实现（llm 模块）

**`llm/transformer/openai/codex/simulation.go`**（新）
- `DefaultVersion()` → `codexDefaultVersion`
- `ApplyClientIdentity(headers, sessionID, version)`：
  - `User-Agent: codex_cli_rs/<version>`
  - `Originator: codex_cli_rs`、`Version: <version>`
  - `Session-Id` / `Session_id` / `Thread-Id` / `Conversation_id`
  - `X-Codex-Window-Id: <session>:0`
  - `X-Codex-Turn-Metadata`（缺失时生成 JSON）
  - `X-Client-Request-Id`（缺失时 UUID）
  - `X-Codex-Beta-Features: remote_compaction_v2`（缺失时）

**`llm/transformer/anthropic/claudecode/simulation.go`**（新）
- `SessionHeader = "X-Claude-Code-Session-Id"`
- `DefaultVersion()` → `2.1.170`
- `ApplyClientIdentity(headers, version)`：
  - `User-Agent: claude-cli/<version> (external, cli)`
  - 复用 `claudeCodeHeaders` 全部 13 项（Anthropic-Beta / Version / X-App / X-Stainless-* 等）

**`llm/transformer/opencode/simulation.go`**（新）
- `DefaultVersion()` → `1.18.31`
- 常量：`ClientHeader` / `RequestHeader` / `ProjectHeader`
- `NewSessionID()`：`ses_` + 12 位 hex 时间 + 14 位 base62
- `SessionIDFromSeed(seed)`：sha256 派生同布局 ID，保证同一会话稳定
- `ApplyClientIdentity(headers, sessionID, version)`：
  - `User-Agent: opencode/<version>`
  - `x-opencode-client: cli`
  - `x-opencode-project: default`
  - `x-opencode-request: <uuid>`
  - `x-opencode-session` / `x-session-id` / `x-session-affinity` → session

### 3.4 后端 — 出站中间件

**`internal/server/orchestrator/client_simulation.go`**（新）
- `applyClientSimulation(outbound)` → `pipeline.OnRawRequest("client-simulation", ...)`
- 读取 `outbound.GetCurrentChannel().Settings.ClientSimulation`
- 会话 ID 解析优先级：
  1. 入站请求中的 CLI session 头（claudecode / codex / opencode）
  2. `shared.GetSessionID(ctx)`
  3. 空 → 由 profile 新生成
- 切换类型前先清理其它 CLI 的 session/归属头（`prepareClientSimulationHeaders`），避免身份混杂
- Codex 分支额外执行 `codex.ScrubClientFingerprintHeaders`
- Claude 分支在 `APIFormat == anthropic/messages` 时追加 query `beta=true`
- OpenCode 分支对非 `ses_` 前缀的 ID 用 `SessionIDFromSeed` 归一化

**`internal/server/orchestrator/orchestrator.go`**
- 注册顺序（关键）：
  `applyUserAgentPassThrough` → **`applyClientSimulation`** → `applyOverrideRequestHeaders`
- 含义：模拟指纹覆盖通用 UA 策略；显式 header override 仍可做最后覆盖

> 测试渠道走同一 orchestrator/pipeline，因此对「测试渠道」请求同样生效。

### 3.5 前端

**新增 `frontend/src/features/channels/components/channels-client-simulation-dialog.tsx`**
- RadioGroup：`none` / `codex` / `claude-code` / `opencode`
- 版本 Input：切换类型时自动填默认值；若当前值等于上一类型默认值也会替换（避免 codex 版本残留到 opencode）
- 协议提示随类型切换显示
- zod 校验：启用模拟时版本不能含空白
- 保存：`useUpdateChannel()` + `mergeChannelSettingsForUpdate(currentRow.settings, { clientSimulation })`
- `none` → 保存 `null`

**`frontend/src/features/channels/components/channels-columns.tsx`**
- 操作列新增菜单项「模拟客户端」（transformOptions 之后），图标 `IconDeviceDesktop`
- 点击 `setCurrentRow(channel)` + `setOpen('clientSimulation')`

**`frontend/src/features/channels/context/channels-context.tsx`**
- `ChannelsDialogType` 追加 `'clientSimulation'`

**`frontend/src/features/channels/components/channels-dialogs.tsx`**
- 导入并渲染 `ChannelsClientSimulationDialog`
- 标准行级 dialog 模式：`currentRow` + `open === 'clientSimulation'` + 关闭时延迟清空 `currentRow`
- 依赖 `useChannelDetails` 自动补齐完整 settings

**`frontend/src/features/channels/data/schema.ts`**
- 新增 `clientSimulationSchema`（type: `codex` / `claude-code` / `opencode`，version 可选）
- `channelSettingsSchema` 新增 `clientSimulation: ...optional().nullable()`

**`frontend/src/features/channels/utils/merge.ts`**
- `mergeChannelSettingsForUpdate` 末尾新增 `pick('clientSimulation', existing?.clientSimulation ?? null)`
- 防止保存其它 dialog 时把该字段丢掉（settings 是整份覆盖语义）

**`frontend/src/features/channels/data/channels.ts`**
- 7 处 settings 选择集新增 `clientSimulation { type version }`：
  - `CREATE_CHANNEL_MUTATION`
  - `DUPLICATE_CHANNEL_MUTATION`
  - `BULK_CREATE_CHANNELS_MUTATION`
  - `UPDATE_CHANNEL_MUTATION`
  - `BULK_IMPORT_CHANNELS_MUTATION`
  - `BULK_UPDATE_CHANNEL_ORDERING_MUTATION`
  - `CHANNEL_QUERY_FULL_NODE_SELECTION`

**i18n**
- `frontend/src/locales/en/channels.json`
- `frontend/src/locales/zh-CN/channels.json`
- 各新增 19 个扁平 key：`channels.dialogs.clientSimulation.*`（action/title/description/client.*/clients.*/version.*/hints.*/errors.invalidVersion）

### 3.6 部署

**`docker-compose.yml`**
- 由 `image: looplj/axonhub:latest` 改为本地构建：
```yaml
build:
  context: .
  dockerfile: Dockerfile
image: axonhub:local
```
- `./config.yml:/app/config.yml:ro` 默认注释（本地无该文件时 Docker 会创建目录导致启动失败）
- 顶部补充构建说明

---

## 4. 运行链路（真实请求）

```
入站请求
  → handler / inbound transformer
  → selectCandidates（选渠道）
  → orchestrator 出站 middleware 链
      applyPassThroughRequestBody
      applyPassThroughRequestHeaders
      applyOverrideRequestBody
      applyUserAgentPassThrough        // 基础 UA
      applyClientSimulation            // ← 新增：CLI 指纹
      applyOverrideRequestHeaders      // 显式覆盖
      ... persistRequestExecution → limiter
  → outbound.TransformRequest（provider 转换，可能已内置指纹）
  → httpclient.Do / DoStream
  → 上游
```

优先级：**显式 header override > 模拟客户端 > UA 透传/默认值**。

---

## 5. 验证记录

| 项目 | 状态 | 说明 |
|------|------|------|
| gqlgen 重新生成 | ✅ 完成 | 后台执行 `go run ../gqlgen/generate.go`，`EXIT=0`；generated.go 已包含新类型与 unmarshal |
| GraphQL schema / 映射 | ✅ 人工核对 | `clientSimulation` 字段与 model 映射存在 |
| 后端常量引用 | ✅ 人工核对 | codex/claudecode/opencode 常量、`shared.GetSessionID`、`llm.APIFormatAnthropicMessage`、`httpclient.Request.Query` 均存在 |
| 前端接线 | ✅ 人工核对 | columns / context / dialogs / schema / merge / 7 处 GraphQL / i18n 全部到位 |
| Go 编译 | ⏳ 未执行 | 仓库规则默认不主动 build/lint；用户将用 Docker 编译验证 |
| 前端 tsc | ⏳ 未执行 | 本地未安装 node_modules |
| Docker 本地构建 | ⏳ 待用户执行 | `docker compose up -d --build` |

已知环境事实：
- 本机为 aarch64 / 4 核；Docker 命令不可用于本会话
- Go 工具链临时安装在 `/tmp/opencode/go-install`（非项目依赖）
- gqlgen 首次运行会删除 `generated.go` / `models_gen.go`，再次生成前建议备份

---

## 6. 使用方式

1. 构建并启动：
```bash
cd /workspace/axonhub-local
docker compose up -d --build
```
2. 打开 `http://localhost:3100`
3. 渠道列表 → 操作列 → **模拟客户端**
4. 选择客户端与版本 → 保存
5. 后续该渠道的真实转发与「测试渠道」都会带上模拟指纹

---

## 7. 待办 / 下一步

- [ ] Docker 编译通过（用户执行）
- [ ] 如编译报错：按报错包定位（重点：`internal/server/orchestrator`、`llm/transformer/*`）
- [ ] 编译通过后做一次真实上游验证：
  - Codex：codex 类型渠道 + 模拟 Codex，测试请求
  - Claude Code：claudecode 类型渠道 + 模拟 Claude Code
  - OpenCode：OpenAI 兼容渠道 + 模拟 OpenCode
- [ ] 视需要补充单元测试：
  - `NormalizeClientSimulation` 边界
  - 三个 `ApplyClientIdentity` 的 header 断言
  - `applyClientSimulation` 的类型切换/会话复用

---

## 8. 风险与注意事项

1. **协议不匹配不会自动纠正**：模拟只改指纹，不换 API 格式。选错协议时上游仍可能拒绝。
2. **版本号直接进 header**：已限制空白/控制字符，避免 header 注入或格式破坏。
3. **gqlgen 生成物体积大**：`internal/server/gql/generated.go` 约 4.2MB，提交时注意 diff 噪声。
4. **settings 整份覆盖**：任何新增 settings 字段都必须同步 `merge.ts`，否则会被其它 dialog 保存时清空。
5. **opencode session 布局**：上游可能严格校验 `ses_` + 时间 + base62；已做归一化，但若上游规则变化需调整 `SessionIDFromSeed`。
6. **Codex Originator**：模拟时固定为 `codex_cli_rs`（与 UA 前缀一致）；如上游要求 `codex-tui`，需在 dialog/实现中扩展。

---

## 9. 关键文件索引

| 层 | 文件 |
|----|------|
| 配置模型 | `internal/objects/channel.go` |
| 校验 | `internal/server/biz/channel.go` |
| GraphQL schema | `internal/server/gql/axonhub.graphql` |
| GraphQL 映射 | `internal/server/gql/gqlgen.yml` |
| 出站中间件 | `internal/server/orchestrator/client_simulation.go` |
| 中间件注册 | `internal/server/orchestrator/orchestrator.go` |
| Codex 指纹 | `llm/transformer/openai/codex/simulation.go` |
| Claude 指纹 | `llm/transformer/anthropic/claudecode/simulation.go` |
| OpenCode 指纹 | `llm/transformer/opencode/simulation.go` |
| 前端 dialog | `frontend/src/features/channels/components/channels-client-simulation-dialog.tsx` |
| 操作列 | `frontend/src/features/channels/components/channels-columns.tsx` |
| Dialog 注册 | `frontend/src/features/channels/components/channels-dialogs.tsx` |
| 前端 schema | `frontend/src/features/channels/data/schema.ts` |
| settings 合并 | `frontend/src/features/channels/utils/merge.ts` |
| GraphQL 查询/变更 | `frontend/src/features/channels/data/channels.ts` |
| 文案 | `frontend/src/locales/{en,zh-CN}/channels.json` |
| 部署 | `docker-compose.yml` / `Dockerfile` |

---

## 2026-10-18 修复：OpenCode 模拟客户端 403

### 现象

渠道类型 `opencode_go`，操作列选择「模拟客户端 → OpenCode」后点击测试，上游返回：

```
OpenCode's free tier can only be used from within OpenCode.
Download it at https://opencode.ai and keep building.
```

### 根因

OpenCode Zen 免费层网关会校验请求的 CLI 指纹，其中 `x-opencode-session` 不只是要求 `ses_` 前缀：

- 真实 CLI 使用 `Identifier.descending()` 编码会话 ID：
  `~(unixMillis << 12 | counter)`，取低 48 位渲染为 12 位 hex，再拼 14 位 base62。
- 编码后的低 12 位必须是 `0xFC0–0xFFF`（counter 0–63）。原实现直接写
  `UnixMilli` 的 hex，低 12 位随机，绝大多数请求会被判定为非 OpenCode 客户端。
- `x-opencode-request` 必须是 `msg_` + 12 hex + 14 base62，且每次请求重新生成。
- 真实 CLI 的 `x-opencode-project` 是 `global`（或项目哈希），不是 `default`。

### 修复内容

`llm/transformer/opencode/simulation.go`：

- 新增 OpenCode 官方 ID 编码：
  - `encodeDescendingID(ts)` = `~(ts*4096 + 1) & 0xFFFFFFFFFFFF`
  - `encodeAscendingID(ts)` 用于 `msg_` 请求 ID
  - counter 固定为 1，保证低 12 位恒为 `0xFFE`
- `NewSessionID()` 使用上述编码 + 14 位 base62。
- `SessionIDFromSeed(seed)` 保持确定性，同时使用合法位型。
- 新增 `NewRequestID()`：`msg_` + 12 hex + 14 base62。
- 新增 `IsValidSessionID()`：校验长度、hex、低 12 位范围和 base62 后缀。
- `ApplyClientIdentity` 改为：UA `opencode/<version>`、`x-opencode-client: cli`、
  `x-opencode-project: global`、`x-opencode-request: msg_...`、
  `x-client-request-id`、`x-opencode-session`/`x-session-id`/`x-session-affinity`。

`internal/server/orchestrator/client_simulation.go`：

- `normalizeOpenCodeSessionID` 不再只看 `ses_` 前缀，改用 `IsValidSessionID`；
  位型不合法时用稳定种子重新生成。
- 清理其它 CLI 指纹时同步删除 `X-Client-Request-Id`。

### 待验证

1. `docker compose up -d --build` 后重新测试 `opencode_go` 渠道。
2. 若仍为同一 403：2026-10-17 起 OpenCode 官方客户端也大面积出现该错误，
   社区判断可能与网关侧故障或出口 IP 有关；可换网络/代理，或改用付费模型验证。
3. 另一类社区反馈称免费层还要求 body `stream: true` 且 tools 数组包含
   `bash`/`glob`/`grep`/`read`。当前实现遵循「不改 body 语义」的决策，未强制注入；
   如确认是 body 闸门，再评估单独开关。

