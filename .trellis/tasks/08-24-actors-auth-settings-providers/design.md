# T03 Design Baseline

## Authority And Entry Gate

父任务 `08-24-python-core-architecture-refactor` 拥有架构、编号决策和跨任务合同。本任务仅在其批准范围内细化实现。

T03 的 Owner 已明确授权 D037 单 writer 实施例外；实现入口要求：

1. D037 已记录，且 T03 是 migration、Core transport、BFF/client、Compose/lock shared paths 的唯一 writer；T02 不会并发修改它们。
2. T03 使用已提交的 T02 `MetaData`/Alembic、`PlatformSettings`、Core transport、BFF/client generator 和 Compose 基础。
3. T03 brief、七项 implement/check manifest、report template 和 dry run 均已审查，exclusive writer 已确认。
4. T02 的 pending implementation evidence 和任何后续 shared-platform/security conflict 必须记入 T03 handoff，并返回父任务 planning，而不是在 T03 内静默替换决定；最终 acceptance 由 T12 负责。

## Scope And Boundaries

- `actors` 拥有 Actor identity、`human | fluctlight` 类型、Owner Human account/session authority 和类型化 Actor references；它不拥有 T04 的 Fluctlight identity/personality 或 T06 的 Conversation membership。
- Python Core 拥有 OwnerAccount、Argon2id credential、opaque Session hash、expiry/revocation、resolved Human Actor、authorization、runtime settings、secret codec、ProviderEndpoint、ModelRole 和 Provider provenance。
- Node BFF 仅拥有 `/auth/*` 浏览器 transport、HttpOnly/Secure/SameSite cookie 和 CSRF/origin 校验。它向 Core 同时转发 opaque human session 和独立 service credential，绝不信任请求 body 中的 Actor ID。
- `.env` 只保存启动/基础设施值；PostgreSQL settings 保存可编辑 Provider URL/key/model 和 policy。敏感值以单 `FLUCTLIGHT_SETTINGS_KEY` 经 vetted AEAD 加密，解密值只存在于 Python Provider adapter 的立即调用范围。
- Provider adapters 只规范化协议、stream/abort、structured response 和有界诊断；它们不持有领域策略、不推断语义、不选择 action，且没有 role/model fallback。
- T03 owns implementation evidence only. T12 re-runs and accepts the required auth/config/provider matrix; T03 does not establish PASS or production readiness.
- Future-only, reserved, and placeholder-only capabilities are excluded from positive acceptance.

## Data And Transport Flow

```text
Browser
  -> BFF cookie + CSRF/origin validation
  -> BFF service credential + opaque session
  -> Core session resolution / Owner authorization
  -> short UoW: Actor/account/session/settings/role state + audit
  -> safe Core result -> BFF safe DTO -> Browser

Owner settings patch
  -> Core validates whole patch and authorizes Owner
  -> AEAD encrypts explicit secret field with purpose/AAD
  -> one UoW persists ciphertext + safe audit
  -> SafeSettingsView exposes configured state only

Role preflight
  -> resolve encrypted key in Python adapter call scope
  -> execute role-specific capability proof
  -> persist safe capability status + checked time
  -> record credential-free provenance / bounded diagnostics
```

The Actor/account/session/settings/role write uses T02's one Unit of Work and a new Alembic revision after `0001_platform`. The task must update readiness's expected revision through the assigned shared-file owner; it cannot leave Core checking only `0001_platform`.

## Shared-File Ownership

After T02 handoff, the one T03 writer owns newly introduced module/test paths. It is the sole approved integration writer for the following shared files when required by the implementation: Core migration environment and next revision, Core route composition/readiness revision, Core/BFF OpenAPI inputs and generated clients, BFF route composition/dependencies/tests, package manifests/locks, and Compose/env documentation. This does not authorize modifying them before the entry gate or concurrently with a T02 writer.

## Failure And Rollback

- Missing/invalid setup token, duplicate owner, invalid password, expired/revoked session, bad CSRF/origin, invalid service credential and Actor spoofing fail independently with bounded responses and no local fallback.
- Missing/wrong settings key, AEAD/purpose mismatch, invalid patch and clear/no-op semantics never write/read plaintext or overwrite a prior secret unintentionally.
- Unassigned/degraded Provider role, invalid schema, stream/abort failure, changed embedding dimensions, timeout and key decryption failure use explicit contract outcomes; no role/model/old-env fallback.
- T03 changes remain internal until T12. Before the evidence handoff, rollback is a new-system code/migration repair or reversion under the shared linear graph, never a legacy compatibility path.
