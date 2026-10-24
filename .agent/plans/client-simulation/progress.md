# 进度日志

## 2026-10-17 会话 1

- [x] 完成 MindOS 启动门禁（bootstrap + 搜索 4 组关键词）
- [x] 定位渠道操作列实现（channels-columns.tsx ActionCell 150 行起）
- [x] 调研 CLIProxyAPI 模拟实现（cloaking/fingerprint 机制）
- [x] 调研 AxonHub 后端注入点（出站 middleware 链 orchestrator.go:289-338）
- [x] 核实前端 dialog 注册机制 + settings 写语义（整份覆盖坑）
- [x] 核实现有三种 CLI transformer（codex / claudecode / opencode）
- [x] 形成方案并与用户对齐

## 2026-10-17/18 会话 2-3

- [x] 后端：`ClientSimulationSettings` + 常量 + 归一化校验（objects）
- [x] 后端：create/update 校验接入（biz/channel.go）
- [x] GraphQL：type/input + ChannelSettings/ChannelSettingsInput 字段 + gqlgen model 映射
- [x] 后端：`applyClientSimulation` 中间件，注册在 UA 透传之后、header override 之前
- [x] llm 模块：codex / claudecode / opencode 三个 `ApplyClientIdentity`
- [x] 前端：操作列菜单项 + dialog + context + dialogs + schema + merge + 7 处 GraphQL settings 片段 + i18n
- [x] gqlgen 重新生成成功（EXIT=0），generated.go/models_gen.go 恢复且包含新类型
- [ ] 编译/类型检查（需用户确认后执行）

## 2026-10-18 会话 4

- 项目目录迁移至 `/workspace/axonhub-local`，全部改动随目录保留
- 新增根目录高保真进度记录：`PROGRESS.md`（需求、决策、逐文件实现、验证状态、待办、风险）
- `docker-compose.yml` 改为本地源码构建（`build: .` + `image: axonhub:local`），并默认注释 `config.yml` 挂载
- 当前待办：用户使用 Docker 编译验证；编译通过后做三类客户端的真实上游验证

## 复现命令

```bash
# Go 工具链（本机临时安装）
export PATH=/tmp/opencode/go-install/go/bin:$PATH
export GOPATH=/tmp/opencode/gopath
export GOCACHE=/tmp/opencode/gocache

# 重新生成 GraphQL 代码（必须从 internal/server/gql 目录运行）
cd internal/server/gql && go run ../gqlgen/generate.go
```

## OpenCode Zen 免费层闸门（2026-10 社区逆向）

- 必需指纹：
  - `User-Agent: opencode/<version>`（大小写不敏感，版本 >= 1.18.0；纯 `opencode/<ver>` 即可）
  - `x-opencode-session: ses_<12 hex><14 base62>`
  - `x-opencode-request: msg_<12 hex><14 base62>`（每次请求新生成）
  - `x-opencode-client: cli`（或 desktop）
  - `x-opencode-project: global`（或 40 位 hex 项目哈希）
- 关键位型：12 位 hex 是 OpenCode `Identifier.descending()` 编码
  `~(unixMillis << 12 | counter)` 的低 48 位；低 12 位必须在 `0xFC0–0xFFF`
  （counter 0–63）。counter=1 → `0xFFE`。时间戳本身几乎不校验（epoch+1ms 也通过）。
- 仅校验 shape 不够，必须校验低 12 位位型，否则随机 hex 前缀会 403。
- 免费层匿名凭据为 `Authorization: Bearer public`；真实 key 也可通过。
- 争议项：部分代理项目称 body 还需 `stream: true` 且 tools 含小写
  `bash`/`glob`/`grep`/`read`；另有逆向称 body 无关。2026-10-17 起官方 2.0.32
  客户端也大面积 403，疑似网关故障/出口 IP，需区分。

## 关键调研结论（详细见 findings.md）

1. AxonHub 已内置 codex / claudecode transformer，含完整指纹（UA、X-Stainless 系列头、X-Codex-* 系列头、fake user_id、system message、?beta=true），但仅限对应 channel type。
2. opencode transformer 只做 X-Opencode-Session 会话头，无 UA / x-opencode-client / project / request。
3. 最优注入点：orchestrator.go 出站链新增 applyClientSimulation，放 applyUserAgentPassThrough 之后、applyOverrideRequestHeaders 之前（早于 persistRequestExecution，测试渠道走同一管线自动生效）。
4. settings 是 JSON 列 + 强类型 GraphQL；前端必须同步 schema/merge/mutations 三处。

## 2026-10-18 会话 5：OpenCode 模拟 403 修复

- 现象：`opencode_go` 渠道 + 模拟客户端 OpenCode，测试返回 `FreeTierError`。
- 根因：`x-opencode-session` 的 12 位 hex 必须是 OpenCode `Identifier.descending()` 编码
  `~(unixMillis<<12|counter)`，低 12 位需落在 `0xFC0–0xFFF`；原实现直接用 UnixMilli，位型随机。
  同时 `x-opencode-request` 需为 `msg_<12hex><14base62>`，`x-opencode-project` 真实值为 `global`。
- 修复：
  - `llm/transformer/opencode/simulation.go`：官方 descending/ascending 编码、counter=1、
    `NewRequestID`、`IsValidSessionID`、`project=global`、`x-client-request-id`。
  - `internal/server/orchestrator/client_simulation.go`：用 `IsValidSessionID` 判断，
    位型非法时按稳定种子重新生成；清理 `X-Client-Request-Id`。
- 待验证：Docker 重建后重测；若仍 403，排查网关 2026-10-17 起的官方客户端集体故障/IP 因素，
  以及社区提到的 body `stream:true` + tools 闸门（当前未改 body）。
