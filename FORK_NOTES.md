# FORK_NOTES — 本 fork（cpa-gateway-extensions）相对上游的改动留档

> 目的：记录 `wzacolemak/CPA-EX` 分支 `cpa-gateway-extensions` 在上游
> [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 之上叠加的全部改动，
> 让未来把上游新 release 合并进来时能快速定位冲突与决策点。
>
> 上游基线：v8.0.45 → 已合并 v8.0.58（2026-11-07）。

## 一、上游合并（rebase）操作流程（已验证两次）

1. 部署宿主上 `source/` 目录不是 git 仓库；真正的仓库镜像在独立的 update 工作目录
   （clone 自本 fork，`origin` = 本 fork，`upstream` = 官方仓库）。
2. 先把部署树的漂移同步进仓库镜像：
   `rsync -rcn --delete --exclude ".git/" --exclude "/claude" --exclude "/auths" <deploy>/source/ <mirror>/`
   （`/claude` 必须根锚定 —— 不锚定会误伤 `internal/translator/antigravity/claude/`；
   dry-run 输出为空 = 无漂移，可直接合并）。
3. `git fetch upstream --tags` → 取最新 release tag → `git merge vX.Y.Z`。
   **用 merge，不要 rebase**：分支已公开推送，rebase 会改写历史。
4. 典型冲突与解法见第五节；`go.mod` 合并后必须核对第六节的依赖清单。
5. 合并提交后 `go build ./...` + 受影响包 `go test`，再推送、替换部署树、重建镜像。

## 二、配置面新增（都在 config.yaml 顶层，上游 schema 迁移时会丢，见第五节第 4 条）

| 配置段 | 作用 | 主要实现文件 |
|---|---|---|
| `api-key-policies` | 客户端 key 命名/策略 | `internal/config/` |
| `daily-cost-quota` | 每客户端 key 滚动成本限额（金额窗口） | `internal/api/daily_cost_quota.go` |
| `model-acl-max-body-size-mb` | 模型 ACL 请求体上限 | `internal/api/model_acl.go` |
| `access:` / `upstream:` / `credentials:` | v8 迁移后保留自定义段的兼容层 | `internal/config/config_v8.go` |
| `upstream.claude.disable-claude-cloak-mode` | 关闭 Claude 伪装，保持透传 | `sdk/cliproxy/` |
| `disable-auto-update-panel: true` | 禁止面板自动更新覆盖本地复合层 | `internal/managementasset/updater.go` |

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
- `ARG GOPROXY` 支持覆盖模块代理；离线友好（去掉 apt-get，复用运行时镜像证书）。
- 支持既有运行时目录布局。

### 面板（`static/management.html`）
- 这是**复合层**：上游某版本的 management.html + 尾部注入脚本（daily-cost 块 +
  外部 zcode 插件的 UI 注入）。**合并时永远保留 fork 侧（ours）**——取上游版本会
  丢掉注入与自定义块。上游面板更新如需吸收，走面板重建流程，不走 git 合并。

### 文档与示例
- `README.md` / `README_EN.md`：网关扩展说明（上游 README 的 fork 增补段）。
- `config.example.yaml`：新增配置段样例。
- `examples/plugin/**/go.mod`：示例插件依赖跟随上游即可（合并冲突取 theirs）。

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
