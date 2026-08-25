# T03 Actor、认证、设置与 Provider

## Goal

交付新系统的 Actor 基础、单 Owner setup/auth/session/authorization、两层配置和敏感设置加密，以及 ProviderEndpoint、六个 Model Role、capability preflight 与 provenance。浏览器只经 BFF 访问，Python Core 是账户、会话、授权、配置和 Provider 凭据的唯一权威。

## Confirmed Facts

- 用户于 2026-08-24 授权在 T02 剩余测试期间准备 T03。
- T02 平台骨架已提交，但其实时任务状态仍为 `in_progress`，没有 PASS report、归档或合并交接证据。`task.json` 是唯一实时状态权威。
- T02 尚未完成的真实 PostgreSQL、迁移/readiness、生成客户端和 Compose 验收会直接影响 T03 的共享 Alembic、Core/BFF、OpenAPI/client 和部署边界。
- 父级 D031/D033 的默认规则不变，但 D037 记录了 Owner 于 2026-08-24 批准的 T03 单 writer 实施例外：T03 可使用已提交的 T02 foundation 实现；T02 不得并发修改 T03 的共享路径。

## Requirements

- D037 下，T03 是唯一写入者，可立即在已提交的 T02 foundation 上实施以下范围；T02 不得并发修改 migration、Core transport、BFF、OpenAPI/client、Compose 或锁文件：
  - `actors` 模块、Owner Human Actor、OwnerAccount、opaque Session、setup/login/logout/revoke/recovery 和 resolved-Actor authorization。
  - `.env` 启动配置与 PostgreSQL runtime settings 的两层边界、单 `FLUCTLIGHT_SETTINGS_KEY` 的 vetted AEAD、write-only safe DTO、审计和 Provider secret resolution。
  - ProviderEndpoint、六个独立 Model Role、capability preflight、明确失败路径和不含秘密/hidden reasoning 的 provenance。
- Core 必须要求彼此独立的 BFF service identity 与 Human session；Node BFF 仅拥有 cookie/CSRF/origin transport，不能持久化或解析认证权威。
- 使用 T02 的单 `MetaData`、线性 Alembic 图、Unit of Work、内部 Core service boundary 和 generated-client 流水线；不创建第二套 migration、metadata、DTO 或 database connection 边界。
- 旧 `server/`、旧 `web/` 和旧 `test/` 只可作为证据，禁止兼容、双写、迁移或分批 cutover。

## Implementation Evidence (Not Acceptance)

- [ ] T03 brief、report template、implement/check manifests 和 dry-run 已经审查；dry run 只确认实现授权文档无歧义，并明确 T02/T12 的剩余风险。
- [ ] D037 的 Owner authorization、exclusive writer 和 T02 非并发共享路径约束已记录；T03 handoff 说明每一处 T02 foundation integration。
- [ ] `task.py start` 前，T03 具有独占 writer，并明确线性迁移、`EXPECTED_REVISION`、OpenAPI/client 生成和 BFF cookie/CSRF 依赖的 owner。
- [ ] 本 child 只记录实现所需的最小格式化、类型、导入、生成物和接口自检；Auth/Configuration/Provider functional contract、real-PostgreSQL、failure、security/redaction 和 browser aggregate validation 全部移交 T12。
- [ ] Future-only、reserved、placeholder-only 功能不在 T03 中做正向验证；相关排除项和 T12 coverage IDs 进入 handoff。
- [ ] 冻结旧实现未修改，未触发产品分批交付或 cutover。
- [ ] 完成后提供包含 changed paths、implementation evidence、remaining risks、migration/OpenAPI artifacts、`acceptance_owner=T12` 和 `acceptance=pending` 的明确 handoff。

## Out Of Scope

- T02 验收、平台缺陷修复或其报告归档本身；T03 也不拥有功能 acceptance。
- 多 Owner、多账号、邀请、角色管理、群聊治理、JWT/localStorage 认证、匿名模式或 localhost-trust fallback。
- Vault/KMS、key ring、per-record data key、旧 SQLite/旧 settings 迁移、Provider 默认/隐式 fallback、产品 cutover。

## Planning State

T03 已获得 D037 的实施例外并将作为唯一 writer 激活。T02 的 pending implementation evidence 仍是必须在 T03 handoff 中记录的共享平台风险，但 T03 不产生 acceptance/PASS；所有功能验收由 T12 重新执行。
