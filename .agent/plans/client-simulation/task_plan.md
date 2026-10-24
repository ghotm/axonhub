# 任务计划：渠道「模拟客户端」功能

## 目标

在渠道列表的「操作」列新增「模拟客户端」入口，弹窗可选择 Codex CLI / Claude Code CLI / OpenCode CLI。
选中后，AxonHub 真正向上游发起请求时，把请求伪装成对应 CLI 客户端（headers / UA / query / 可选 body），
用于绕过只允许特定 CLI 访问的上游 API 限制。

## 阶段

| 阶段 | 内容 | 状态 |
|------|------|------|
| 1 | 调研：CLIProxyAPI 的 Codex/Claude 客户端模拟实现 | complete |
| 2 | 调研：AxonHub 渠道 settings 模型、前端操作列/dialog 注册机制 | complete |
| 3 | 调研：AxonHub 出站请求管线与 header/body 注入点 | complete |
| 4 | 设计：配置模型、持久化、UI 交互、后端注入架构 | complete |
| 5 | 与用户对齐方案（关键决策确认） | complete |
| 6 | 实现后端模拟层：settings 字段 + GraphQL + 校验 + applyClientSimulation 中间件 | complete |
| 7 | 实现前端：操作列入口 + dialog + schema/merge/mutations + i18n | complete |
| 8 | 重新生成 gqlgen、测试与验证 | complete |

### 阶段 8 结果

- gqlgen 已后台重跑成功（EXIT=0），`generated.go` / `models_gen.go` 已恢复并包含 `ClientSimulationSettings`、`clientSimulation` 字段与 unmarshal 逻辑。
- 已人工核对：GraphQL schema/映射、后端校验、中间件注册位置（305 → 311 → 312）、三个 `simulation.go` 的常量引用、前端 dialog/context/dialogs/columns/schema/merge/mutations/i18n 全部到位。
- 未运行 `go build` / lint（遵循仓库规则，需用户明确要求才执行编译类验证）。
- 前端未安装 node_modules，未运行 `tsc`。

## 设计草案（阶段 4 结论）

### 配置模型
`channel.settings.clientSimulation`（JSON 列新增字段，无需迁移）：

```go
type ClientSimulationType string // "none" | "codex" | "claude-code" | "opencode"

type ClientSimulationSettings struct {
    Type    ClientSimulationType `json:"type"`              // 默认 "none"
    Version string               `json:"version,omitempty"` // 可选版本覆盖（codex 版本 / claude-cli 版本 / opencode 版本）
}
```

- 作用范围：真实转发 + 测试渠道（测试走同一 orchestrator，自动生效）。
- 优先级：applyUserAgentPassThrough（先设基础 UA）→ applyClientSimulation（覆盖为 CLI 指纹）→ applyOverrideRequestHeaders（显式 override 最后盖）。

### 后端注入
- 新文件 `internal/server/orchestrator/client_simulation.go`：`applyClientSimulation(outbound)` 中间件。
- 读 `outbound.GetCurrentChannel().Settings.ClientSimulation`，按类型调用 profile：
  - **Codex**：复用 `llm/transformer/openai/codex` 的 constants/headers（UA `codex_cli_rs/<ver>`、Originator、Session-Id、Thread-Id、Window-Id、Turn-Metadata、Client-Request-Id、Beta-Features、Version、Conversation_id；入站透传 Hashable headers）
  - **Claude Code**：复用 `llm/transformer/anthropic/claudecode` constants（13 个 X-Stainless/Anthropic-* 头、UA `claude-cli/<ver> (external, cli)`、`?beta=true`、可选 metadata.user_id）
  - **OpenCode**：UA + X-Opencode-Session/Client/Project/Request + X-Session-Id（补齐现有 transformer 缺失的指纹）
- 注册位置：orchestrator.go:305 之后、306（applyOverrideRequestHeaders）之前。

### 前端
- columns 菜单项「模拟客户端」→ `setOpen('clientSimulation')`
- 新 dialog `channels-client-simulation-dialog.tsx`（单选：不模拟/Codex/Claude Code/OpenCode + 版本输入）
- context union / dialogs 渲染 / schema / merge / mutations / i18n 五处同步

## 已确认决策（阶段 5）

1. 模拟深度：仅 header/UA/query 指纹，不动 body 语义。
2. 完全独立：任意渠道操作列都显示入口；配置叠加在现有 transformer 之上；未配置保持原行为。
3. 版本号 dialog 可配置，默认预填内置版本（codex 0.159.0 / claude-cli 2.1.170 / opencode 1.18.31）。
4. 协议边界：不改渠道 API 格式/endpoint 路径，dialog 内给出协议提示。

## 已落地实现（阶段 6-7）

- 后端：
  - `internal/objects/channel.go`：`ClientSimulationSettings{Type,Version}` + 类型常量 + `DefaultClientSimulationVersion` + `NormalizeClientSimulation`
  - `internal/server/biz/channel.go`：create/update 校验归一化
  - `internal/server/gql/axonhub.graphql` + `gqlgen.yml`：新类型与字段映射
  - `internal/server/orchestrator/client_simulation.go`：`applyClientSimulation` 中间件（注册位置在 applyUserAgentPassThrough 之后、applyOverrideRequestHeaders 之前）
  - `llm/transformer/{codex,claudecode,opencode}/simulation.go`：三个 `ApplyClientIdentity` + opencode `NewSessionID`/`SessionIDFromSeed`
- 前端：
  - `channels-client-simulation-dialog.tsx`（RadioGroup 单选 + 版本输入 + 协议提示）
  - context union / dialogs 渲染 / columns 菜单 / schema / merge / channels.ts 7 处 mutations/queries / i18n en+zh-CN

## 遇到的错误

| 错误 | 尝试次数 | 解决方案 |
|------|---------|---------|
| 环境无 go 工具链，gqlgen 无法生成 | 3 | 下载 go1.26.9 linux-arm64 到 /tmp/opencode/go-install，导出 PATH/GOPATH/GOCACHE |
| gqlgen 首次运行超时且中途删除 generated.go/models_gen.go | 2 | git checkout 恢复 + 备份到 /tmp/opencode/gql-backup，后台重跑 |
| channels.ts 的 settings 片段有 7 处相同结构，edit 工具匹配到 3 处 | 1 | Python 脚本按 providerQuota 块逐处插入 clientSimulation，随后清理 1 处重复 |