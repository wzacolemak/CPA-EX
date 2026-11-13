# FORK_NOTES — 本 fork（cpa-gateway-extensions）相对上游的改动留档

> 目的：记录 `wzacolemak/CPA-EX` 分支 `cpa-gateway-extensions` 在上游
> [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 之上叠加的全部改动，
> 让未来把上游新 release 合并进来时能快速定位冲突与决策点。
>
> 上游基线：v8.0.45 → v8.0.58（2026-11-07）→ **v8.0.69（2026-11-13）**。

## 一、上游合并（rebase）操作流程（已验证三次）

> 术语说明：本文件标题沿用用户习惯叫法「rebase」，实际执行的是 **merge**（见第 3 步）。

1. 部署宿主上 `source/` 目录不是 git 仓库；真正的仓库镜像在独立的 update 工作目录
   （clone 自本 fork，`origin` = 本 fork，`upstream` = 官方仓库）。
2. 先把部署树的漂移同步进仓库镜像：
   `rsync -rcn --delete --exclude ".git/" --exclude "/claude" --exclude "/auths" <deploy>/source/ <mirror>/`
   （`/claude` 必须根锚定 —— 不锚定会误伤 `internal/translator/antigravity/claude/`；
   dry-run 输出为空 = 无漂移，可直接合并）。
3. **先备份分支**：`git branch -f backup-pre-vX.Y.Z-$(date +%Y%m%d-%H%M%S) HEAD`。
4. `git fetch upstream --tags` → 取最新 release tag → `git merge vX.Y.Z`。
   **用 merge，不要 rebase**：分支已公开推送，rebase 会改写历史。
5. 典型冲突与解法见第五节；`go.mod` 合并后必须核对第六节的依赖清单。
6. **确认 fork 补丁未被上游冲掉**（每次合并后固定检查，缺一个就是事故）：
   ```bash
   grep -c cpaThinkingSignature internal/translator/openai/claude/openai_claude_response.go
   grep -c antigravityRefusalNotice internal/translator/antigravity/claude/antigravity_claude_response.go
   grep -c 'access`,`api-keys' static/management.html
   grep -c ca-certificates.crt Dockerfile
   ls internal/api/model_acl.go internal/api/daily_cost_quota.go
   grep -n api-key-policies internal/config/config_v8.go
   ```
   全部非零才继续。
7. 合并提交后 `go build ./...` + 受影响包 `go test`；再跑一次 `go test ./...`
   确认没有新增失败（目前已知唯一既有失败是
   `internal/client/codex/live TestPionMediaRelayBridgesAudioAndDataChannel`，
   上游自带，与 fork 无关）。
8. 推送 → 替换部署树（rsync 同第 2 步但去掉 `-n`）→ 更新 `docker-compose.yml` 的
   `COMMIT`/`BUILD_DATE`/`image:` → `docker compose build && up -d` → 按第七节验收。

### 判据：什么时候该取 theirs（上游）而不是死守 fork

合并冲突不是「一律保留 fork」。**当上游独立修好了同一个问题时，取上游版本并删除
fork 侧补丁**，否则会留下两套机制互相打架。v8.0.69 的实例：

- `openai_responses_interrupt_test.go`：fork 用 `canceled chan struct{}` 修 flaky，
  上游同样加了 `canceledCh` + `cancelOnce`。→ 取 theirs。
- `TestServerResponsesWebsocketReauthorizesEachTurn`：fork 曾给测试服务器注入
  `discardRequestLogger` 来躲开 `t.TempDir` 清理竞态；上游改为用 `requests.Wait()`
  在 cleanup 前排空被 hijack 的 handler，**并且新增了「日志必须真的落盘」的断言**。
  此时保留 fork 的 discard logger 会静默废掉这条断言。→ 撤销 fork 补丁，取上游。
- `nocopy_invariant_test.go`：两侧都给同一文件加了白名单条目。→ 取 theirs
  （等价且措辞更好），不要留下重复 id。

## 二、配置面新增（都在 config.yaml 顶层，上游 schema 迁移时会丢，见第五节第 4 条）

| 配置段 | 作用 | 主要实现文件 |
|---|---|---|
| `api-key-policies` | 客户端 key 命名/策略 | `internal/config/` |
| `daily-cost-quota` | 每客户端 key 滚动成本限额（金额窗口） | `internal/api/daily_cost_quota.go` |
| `model-acl-max-body-size-mb` | 模型 ACL 请求体上限 | `internal/api/model_acl.go` |
| `access:` / `upstream:` / `credentials:` | v8 迁移后保留自定义段的兼容层 | `internal/config/config_v8.go` |
| `upstream.claude.disable-claude-cloak-mode` | 关闭 Claude 伪装，保持透传 | `sdk/cliproxy/` |
| `disable-auto-update-panel: true` | 禁止面板自动更新覆盖本地复合层 | `internal/managementasset/updater.go` |
| `disabled: true`（渠道 key 级） | 保留凭据但不参与轮询，面板可开关 | `internal/watcher/synthesizer/config.go`、`internal/api/handlers/management/config_apikey_disable.go` |

### 渠道 key 的 disabled 开关（2026-11-12 补齐）

`gemini-api-key` / `interactions-api-key` / `claude-api-key` / `codex-api-key` /
`xai-api-key` / `meta-api-key` / `vertex-api-key` 的每个 key，以及
`openai-compatibility.<provider>.keys[]` 的每个 key，都支持 `disabled: true`：

- 合成期置 `auth.Disabled = true` + `auth.Status = StatusDisabled`，凭据仍在列表里、
  仍被面板显示，但路由与冷却逻辑全部跳过（`selector.go` / `conductor_cooldown.go`）。
- 开关走 `PATCH /v8/management/credentials/status`，落盘到 config.yaml。
- **曾经的坑**：`openai-compatibility` 下的 key 会被合成为 config API key auth，
  面板能列出但 PATCH 返回 404，永远关不掉——因为 `toggleConfigAPIKeyDisabled`
  没遍历该 provider。已补 `toggleOpenAICompatibilityKeyDisabled`，
  按合成器完全一致的 auth ID 重建规则匹配（含 connection-pool dedup id）。
- 旧版本的「禁用」是往 `excluded-models` 追加 `*`；开启时 `withoutLegacyDisablePattern`
  会清掉这个遗留标记，真实模型 id 保留。

## 二之二、openai-compatibility 上游的思考强度（2026-11-13 实测）

**问题**：本机反代上游（`aistudio-web` → AIStudio2API）挂了 `gemini-4-argon`。客户端若走
Anthropic 协议（`/v1/messages`），CPA 转译为 OpenAI 兼容请求时**会丢弃 `thinking` 参数**，
上游回落到自身默认思考档；而 `gemini-4-argon` 的上游默认是 `medium`，该档**不产生任何思考**
（表现为秒回、无思考链）。

**实测数据**（同一提示词）：

| 入口协议 | 请求参数 | reasoning_tokens | 耗时 |
|---|---|---|---|
| Anthropic `/v1/messages` | `thinking.enabled(budget=4000)` | **0** | 8s |
| Anthropic `/v1/messages` | 无 thinking | **0** | 8s |
| OpenAI `/v1/chat/completions` | `reasoning_effort=high` | ~1020 | 23s |
| OpenAI + 模型名带 `(high)` | 无需额外参数 | ~1009 | 26s |

补充：`gemini-4-argon` 的 `low` / `medium` 档上游均**不产生思考**（rt=0），只有 `high` 真思考。
`gemini-4-hydrogen` 四档（low/medium/high/xhigh）都有效，行为与 argon 不同。

**解法**：把括号后缀写在 `openai-compatibility` 的**上游 `name`** 上，客户端继续用 alias 原名：

```yaml
- name: aistudio-web
  base-url: "http://aistudio2api:2048/v1"
  models:
    # 括号在 name 上：CPA 路由前剥掉括号并按档位注入 reasoning_effort
    - name: "gemini-4-argon(high)"
      alias: "gemini-4-argon-aistudio"        # 客户端/DSH 里用的仍是这个名字
    - name: "gemini-4-hydrogen"
      alias: "gemini-4-hydrogen-aistudio"
```

**要点**：
1. `internal/thinking/suffix.go` 的括号解析对 `openai-compatibility` 上游同样生效，
   不限于 OAuth 通道——官方文档只描述了客户端侧用法，未提上游 `name` 侧。
2. 上游 `name` 带括号后，`/v1/models` 里出现的是 `alias`，客户端无感知。
3. 反代侧（AIStudio2API）收到的是剥括号后的 `gemini-4-argon` + `reasoning_effort: high`。
4. 需要多档时每档配一条 alias，例如 `gemini-4-argon(low)` → `gemini-4-argon-aistudio-low`。

**验证方法**（不经 DSH，直接打 CPA）：

```bash
curl -s http://127.0.0.1:3003/v1/messages \
  -H "x-api-key: $CPA_API_KEY" -H 'anthropic-version: 2023-06-01' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gemini-4-argon-aistudio","max_tokens":4000,
       "messages":[{"role":"user","content":"简要分析一个概率问题"}]}' \
| jq '.usage, [.content[] | select(.type=="thinking")] | length'
```
期望：`output_tokens` 显著大于 0 且 thinking 块数 ≥ 1。

## 三、源码改动分区（相对上游累计 ~2.8k 行）

### 网关扩展（本 fork 存在的主因）
- `internal/api/model_acl.go` (+174)：按客户端 key 的模型 ACL（含 gjson 解析请求体）。
  **注意：新增了 `github.com/tidwall/gjson` 依赖**，上游 go.mod 不含它。
- `internal/api/daily_cost_quota.go` (+272)：滚动成本配额中间件与统计。
- `internal/api/server_routes.go` (+202)：扩展路由注册（管理扩展 + usage-keeper 代理）。
- `internal/api/handlers/management/config_auth_index.go`：OpenAI-compat 渠道 key 的
  auth-index 稳定回退（`stableOpenAICompatAuthIndex`）—— 上游管理 API 对 live index
  查不到的 key 返回空，回退保证外部用量系统身份稳定。合并时与上游的
  connection-pool dedup 逻辑取**并集**。
- `internal/api/handlers/management/usage_keeper_embed.go` (+166)：usage-keeper 嵌入代理。
- `internal/api/handlers/management/daily_cost_quota.go` (+98)：限额管理端点。
- `sdk/cliproxy/service_lifecycle.go` (+20)：生命周期扩展点。
- `internal/api/server.go` / `server_management.go` / `server_reload.go`：小扩展。

### 翻译层修复
- `internal/translator/antigravity/claude/antigravity_claude_response.go`：
  content-filter finishReason（SAFETY/RECITATION/PROHIBITED_CONTENT/SPII/BLOCKLIST/
  IMAGE_SAFETY/MALFORMED_*）映射为 `refusal` 并注入人类可读说明，避免客户端收到
  静默空响应。上游 ≥ v8.0.58 的 `resolveAntigravityClaudeStopReason` 已收录同一原因集；
  fork 在其上保留**流式与非流式的 notice 注入**（`antigravityRefusalNotice`）。
  合并时：helper 取上游，notice 注入按新结构（ojson/responseDoc）重放。

### 构建（Dockerfile）
- `ENV GOTOOLCHAIN=auto`（上游 go directive 常超前基础镜像自带工具链）。
- `ARG GOPROXY` 支持覆盖模块代理；离线友好（去掉 apt-get 安装步骤）。
- **运行阶段必须自带 CA 根证书**（2026-11-13 事故）：`RUNTIME_IMAGE` 默认
  `debian:bookworm` **没有 `/etc/ssl/certs/`**，Go 的 TLS 于是拒绝一切公网 HTTPS
  上游，报 `certificate signed by unknown authority`。表现为**所有渠道同时失效**，
  极易误判为网络/代理故障（实际排查中先怀疑了机房链路、节点池、MTU，全部无关）。
  解法：`COPY --from=builder /etc/ssl/certs/ca-certificates.crt`（builder 阶段
  `golang:1.26-bookworm` 自带），并在镜像末尾加构建期断言，让缺证书在**构建期**
  失败而不是上线后第一个请求才暴露：
  ```
  RUN test -s /etc/ssl/certs/ca-certificates.crt || { echo "ERROR: ..." >&2; exit 1; }
  ```
  （内容断言 `grep -q "BEGIN CERTIFICATE"` 需 `command -v grep` 保护，避免精简
  基础镜像误报。）
- **compose 的 build args 会覆盖手工 tag**：`docker compose up` 若发现 `build:`
  段就会重建镜像并按 compose 内的 `COMMIT`/`BUILD_DATE` 打标。手工
  `docker build -t ...` 的产物会被静默替换（2026-11-12 因此上线了一个缺证书的
  镜像）。改代码后**必须同步更新 `docker-compose.yml` 的 `COMMIT`、`BUILD_DATE`
  与 `image:` tag**，并以 compose 构建为准。
- 支持既有运行时目录布局。

### 面板（`static/management.html`）
- 这是**复合层**：上游某版本的 management.html + 尾部注入脚本（daily-cost 块 +
  外部 zcode 插件的 UI 注入）。**合并时永远保留 fork 侧（ours）**——取上游版本会
  丢掉注入与自定义块。上游面板更新如需吸收，走面板重建流程，不走 git 合并。

### 文档与示例
- `README.md` / `README_EN.md`：网关扩展说明（上游 README 的 fork 增补段）。
- `config.example.yaml`：新增配置段样例。
- `examples/plugin/**/go.mod`：示例插件依赖跟随上游即可（合并冲突取 theirs）。

## 三之二、上游版本演进记录

### v8.0.58 → v8.0.69（2026-11-13 合并，94 个非 merge 上游提交 / 111 个含 merge）

与本 fork 交集最大、升级时必须留意的几项：

| 上游改动 | 对本 fork 的影响 |
|---|---|
| `feat(tunnel)`: 新增多租户 WebSocket 数据交换隧道 `/v1/tunnel`（+服务类型元数据、`/v1/tunnel` 系列路由） | 纯新增，无冲突。但新增了 `/v1/tunnel` 路由，若入口 nginx 有限流/白名单需同步放行 |
| `feat(api)`: `server.shutdown-timeout` 可选优雅退出 | 纯新增；本 fork 的 `sdk/cliproxy/service_lifecycle.go` 扩展点不受影响 |
| `fix(gemini)`: 跨工具调用轮次保留 thought signature | 与本 fork 的 `cpaThinkingSignature` 属**不同层**（上游管 Gemini→Claude 真签名透传，fork 管 OpenAI→Claude 无签名铸造）。已确认共存 |
| `fix(thinking)`: 校验 native responses 的 reasoning efforts | 与本 fork 翻译层改动无重叠 |
| `test(api)`: WebSocket 重授权测试新增「request log=true/false」双档 + 断言日志落盘 | **直接冲突**：本 fork 曾用 discardRequestLogger 规避竞态，该断言要求日志真落盘。已撤销 fork 补丁（见第一节判据） |
| `fix(auth)`: 加权池公平性与强制冷却、refresh 任务退避 | 上游修复了 `selector_weighted.go`，并自行登记进 nocopy 白名单 |
| `feat(models)`: Devin 目录与 Gemini 变体更新 | 仅数据 |
| `feat(cli)`: config 加载/TUI/启动失败返回非零退出码 | 部署脚本若依赖退出码会有行为改善（以前失败也可能返回 0） |

冲突实况：3 个文件（`nocopy_invariant_test.go`、`config_v8_test.go`、
`openai_responses_interrupt_test.go`），全部按第一节判据处理。`internal/api/server.go`、
`server_routes.go`、`config_types.go`、`config_v8.go`、`openai_claude_response.go`
等「历史悠久的高发文件」本次**自动合并成功**。

## 四、外部插件（不在本仓库源码树，但升级时必须一起验证）

| 插件 | 状态 | ABI 依赖 |
|---|---|---|
| zcode（bigmodel/zcode ultra 执行器） | 生产启用 | `sdk/pluginapi` + `sdk/pluginabi`，声明 schema 7 |
| dsml-fixer（DSML 文本→tool_calls 还原） | 已禁用（enabled:false，待修复流式透传 bug） | 同上 |

- 上游/本 fork 的 plugin ABI 自 schema 7 → 10 是**纯增量**（新增 opt-in 能力：v8 请求体
  patch、v9 多 provider 模型注册、v10 多 auth provider），宿主按插件声明版本自适应，
  旧 .so 无需重编。若未来出现 wire 字段改名（而非新增），必须重编两个 .so。
- 插件以 bind-mount 方式挂载（`plugins/` 目录），镜像重建不影响它们；CPA 重启即重新加载。

## 五、冲突高发文件与解法配方

1. `internal/api/server.go` — 上游常改中间件顺序。解法：union（fork 的
   `CPATraceIDMiddleware` / `maxRequestBodyBytesMiddleware` / extraMiddleware 全保留，
   顺序跟随上游注释意图）。
2. `internal/config/sdk_config.go` — 上游扩展 ClientConfig。解法：union（fork 的字段 + 上游新结构）。
3. `internal/api/handlers/management/handler.go` / `config_auth_index.go` — 上游重构
   auth-index 生成。解法：上游新逻辑打底 + fork 的 `stableOpenAICompatAuthIndex` 回退。
4. **配置迁移丢段事故（已发生过）**：上游 schema 迁移或任何不认识自定义段的工具重写
   config.yaml 会静默丢掉第二节的自定义段。合并后部署前：diff 线上 config.yaml 与
   `config_v8.go` 迁移逻辑，确认四段仍在。
5. `go.mod` — 上游依赖树变化会挤掉 fork 独有依赖。合并后跑
   `go build ./...`，缺 `tidwall/gjson` 就 `go get` 补回（fork 唯一新增直接依赖）。
6. `internal/translator/antigravity/claude/` — 见第三节；`*_test.go` 同步需要 gjson import。
7. `examples/plugin/**/go.mod` — 永远取上游（theirs），fork 不维护示例。

## 六、升级后部署验证清单

1. 容器日志：`plugin loaded plugin_id=zcode`（以及按需 dsml-fixer）出现、无 panic。
2. `GET /v1/models` 数量与内容正常（含外部插件注册的模型与别名）。
3. 流式冒烟：Anthropic 协议 `/v1/messages`（glm 系列）必须以 `message_stop` 收尾；
   OpenAI 协议 `/v1/chat/completions`（OpenCode Go 渠道）finish_reason 正常。
4. zcode 插件端点：`/v0/resource/plugins/zcode/summary` 返回 JSON（非 HTML 错误页）。
5. usage-keeper 健康：配额缓存含 zcode 三窗口行；GLM 事件的 input 口径 = 折叠后
   （input ≥ cache_read，total = input + output）。
6. 管理面板可登录、认证文件页正常（复合层 md5 与镜像内一致）。
7. 每日成本限额端点 `/v0/management/daily-cost-quota` 返回 enabled + limits。
8. **CA 根证书存在**（缺了就是全线 403/unknown authority）：
   `docker exec cli-proxy-api sh -c 'ls -l /etc/ssl/certs/ca-certificates.crt'`
   且镜像内 `head -1` 为 `-----BEGIN CERTIFICATE-----`。
9. **镜像 commit 与 compose 一致**：`docker exec cli-proxy-api ./CLIProxyAPI --version`
   的 Commit 必须等于 `docker-compose.yml` 里的 `COMMIT`（不一致说明 compose
   重建覆盖了手工镜像，见第三节「构建」条目）。
10. 各渠道逐一实测（不只测一个）：至少覆盖 `dsfgo`、`glm-whz-*`、`gpt-whz-*`、
    `glm-aaa`、`gemini-4-argon-*`，确认全 200。**注意区分真实故障与已知的
   账户/地区问题**：`gemini-4-argon-high` 的 503 是 Google 账户 ToS 封禁，
    `gemini-4-argon-aistudio` 的 403 `Region not supported` 是出口地区问题，
   二者都不是升级引入的回归。
11. 上游新增路由按需放行：`/v1/tunnel`（WebSocket）。若入口 nginx 有限流规则，
   确认它不会被误拦或误限。

## 面板 v8 适配补丁（2026-11-07，static/management.html 两处中文件改动）

背景：config 自 2026-10-28 起为 v8 布局（客户端 key 在 access.api-keys，顶层 api-keys 是上游渠道组），
但上游面板的「API 密钥列表」区块只认 v7 顶层列表——显示为空，且在该区块保存会把顶层 api-keys
写成字符串数组 = 清空全部上游渠道组（kc1 2026-11-07 21:46 渠道丢失事故的根因）。

改动（压缩代码，标识符随上游构建变化，重放时按语义定位）：
1. _P(e)（apiKeysText 加载）：最前面插入 access.api-keys 优先读取——
   `let _a8=mP(e?.access)?.[`api-keys`];if(Array.isArray(_a8))return gP(_a8);`
2. applyVisualChangesToYaml 的 api-keys 分支：`setIn([`api-keys`],e)` / `deleteIn([`api-keys`])`
   全部改写为 `access`,`api-keys` 路径，不再触碰顶层 api-keys。

部署配套（kc1）：docker-compose 挂载 ./management.html:/CLIProxyAPI/static/management.html
（单文件 bind mount——host 侧修改必须原地写）；config.yaml management.disable-auto-update-panel: true
（v8.0.58 起此开关已接线，updater.go:120）。上游面板更新时本补丁需重放。
