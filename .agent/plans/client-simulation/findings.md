# 调研发现

## MindOS 背景

- 本机存在容器 `cli-proxy-api-plus`（CLI 代理 API 扩展，曾占用 8317 端口）。
- 用户画像记录：`GoProxy / CLIProxyAPI（AxonHub）→ Go 代理工具链，渠道覆盖设置等功能`。
- MindOS 中没有「模拟客户端」的既有实现记录。

## AxonHub 前端

- 渠道列表操作列：`frontend/src/features/channels/components/channels-columns.tsx` 的 `ActionCell`。
- 操作项通过 `useChannels()` 的 `setCurrentRow(channel)` + `setOpen('<dialogKey>')` 打开弹窗。
- 已有 dialog key：test / testHistory / duplicate / modelMapping / price / overrides / proxy / transformOptions / rateLimit / endpoints / keyManagement / availability / disabledAPIKeys / errorResolved / archive / delete。
- 需要查看 `channels-dialogs.tsx` 了解注册方式。

## OpenCode Zen 免费层闸门（2026-10 社区逆向）

- 必需指纹：
  - `User-Agent: opencode/<version>`（大小写不敏感，版本 >= 1.18.0；纯 `opencode/<ver>` 即可）
  - `x-opencode-session: ses_<12 hex><14 base62>`
  - `x-opencode-request: msg_<12 hex><14 base62>`（每次请求新生成）
  - `x-opencode-client: cli`（或 desktop）
  - `x-opencode-project: global`（或 40 位 hex 项目哈希）
- 关键位型：12 位 hex 是 OpenCode `Identifier.descending()` 编码
  `~(unixMillis << 12 | counter)` 的低 48 位；低 12 位必须在 `0xFC0–0xFFF`
  （counter 0–63）。counter=1 → `0xFFE`。时间戳本身几乎不校验。
- 仅校验 shape 不够，必须校验低 12 位位型，否则随机 hex 前缀会 403。
- 匿名免费层凭据为 `Authorization: Bearer public`；真实 key 也可通过。
- 争议项：部分代理项目称 body 还需 `stream: true` 且 tools 含小写
  `bash`/`glob`/`grep`/`read`；另有逆向称 body 无关。2026-10-17 起官方 2.0.32
  客户端也大面积 403，疑似网关故障/出口 IP，需区分。

