# CLI Proxy API Gateway Extensions

[中文说明](README.md) | English

This is a self-hosted gateway extension built on [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). It keeps the upstream OpenAI, Responses, Gemini, Claude, Codex, and provider support, then adds the controls needed when several clients share one gateway.

It is not a rewrite of CPA. Upstream still owns protocol compatibility, authentication, account pools, and model execution. This repository focuses on client API-key governance and operations.

## Changes in this fork

- Per-client API-key names and model allowlists. A restricted key can request only its allowed models, and `/v1/models` is filtered to the same visible set.
- Configurable request-body limit for model ACL inspection through `model-acl-max-body-size-mb`. Unrestricted keys are not buffered for this check. Keep the limit aligned with the body limit in nginx or another reverse proxy.
- Rolling 24-hour cost quotas per client key. CPA reads `range=24h` aggregates from a separately deployed CPA Usage Keeper, with configurable refresh and fail-open behaviour.
- API-key names can be synchronized to Usage Keeper aliases.
- Provider `display-name` values for the management UI, model tester, and usage statistics. An OpenAI Compatibility provider's `name` remains its routing identifier.
- Management-panel support for API-key model permissions and quotas, plus embedded usage statistics and model testing behind the existing CPA management session.
- Polling fallback for configuration hot reload when a Docker single-file bind mount does not produce filesystem events.
- A configurable Docker runtime base image through `RUNTIME_IMAGE`.

## Configuration

Start with [config.example.yaml](config.example.yaml). Do not commit production configuration, `auths/`, logs, or Usage Keeper data.

```yaml
api-keys:
  - "team-a-secret"

api-key-policies:
  - key: "team-a-secret"
    name: "team-a"
    allowed-models:
      - "gpt-5.6-*"
      - "claude-*"

model-acl-max-body-size-mb: 64

daily-cost-quota:
  enabled: true
  keeper-base-url: "http://cpa-usage-keeper:8080"
  keeper-password-env: "CPA_USAGE_KEEPER_PASSWORD"
  refresh-seconds: 15
  fail-open: false
  limits:
    - key: "team-a-secret"
      limit: 10.00
```

`allowed-models` accepts model match patterns. Keys without a restriction keep access to all models unless `api-key-default-policy: deny-all` is set. See [config.example.yaml](config.example.yaml) for all fields.

For a provider display name:

```yaml
openai-compatibility:
  - name: "company-upstream"       # Routing identifier; keep it stable.
    display-name: "Company OpenAI"  # UI and statistics label.
    base-url: "https://example.com/v1"
    api-key: "${UPSTREAM_API_KEY}"
```

## Management panel and companion services

The management panel edits CPA configuration and creates short-lived embed sessions. Usage statistics and model testing appear inside it, but their services should stay on a private network.

- [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper) stores and aggregates usage data and supplies the rolling-cost quota data.
- The model-testing service is separate from this repository. It provides the model list and test interface embedded in the management panel.

The browser receives a short-lived embed capability, not a standalone Keeper password. Restrict access to the management port and keep both companion services reachable only from CPA.

## Upstream documentation

- [CLIProxyAPI guides](https://help.router-for.me/)
- [Management API](https://help.router-for.me/management/api)
- [SDK documentation](docs/sdk-usage.md)

When rebasing or upgrading, use the official release as the base and then transplant these gateway changes. In particular, retain upstream protection that prevents config API-key channels from persisting runtime state into `auth-dir`.

## Attribution and license

This repository is released under the [MIT License](LICENSE) and retains upstream copyright and license notices.

- [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), the upstream project, MIT License.
- [Willxup/cpa-usage-keeper](https://github.com/Willxup/cpa-usage-keeper), the usage and cost aggregation service, MIT License. It is deployed separately and is not distributed with this repository.
- [router-for-me/Cli-Proxy-API-Management-Center](https://github.com/router-for-me/Cli-Proxy-API-Management-Center), the source of the management-panel asset, MIT License. This fork adds the API-key permission, quota, and embedded-page entry points on top of it.
