# Fact-only 测试渠道与稳定回退

功能分支为 `codex/fact-only-reply-vision`。首次构建使用功能分支 push；仅在分支中的 workflow 存在并不保证默认分支 UI 立刻注册新按钮。已有工作流可选择该分支手动运行，实际 checkout SHA 必须与 run SHA 相同。

沿用 Docker Hub 的 `fluctlight-core-go`、`fluctlight-web` 两个镜像仓库与原认证。两个镜像先分别发布 `fact-only-<完整SHA>-<runId>-<attempt>`，全部成功后核对远端分支 HEAD，再更新 `fact-only-latest`。发布清单在该 run 的 `fact-only-release-<runId>-<attempt>` artifact，包含各镜像源 SHA、ref、tag、digest、构建 URL 和别名晋升状态。只有 `status=published` 可作为完整测试版本；`promotion_failed` 可能已更新其中一个别名，选清单中的不可变 digest，不能把两仓别名当作原子版本。

PR 不发布镜像。测试任务没有稳定 `latest` 写入路径。现有 master 工作流仅 push 发布稳定镜像，当前 master 的手动 dispatch 不发布；回退可复用已成功的 master push 构建产物或重跑其 run。功能分支改动未合入 master 前，不能把新的工作流行为当作 master 已具备的入口。

## 切换测试版本

先保存稳定 Core/Web **实际** tag/digest、settings 私有备份和部署 env。镜像 namespace 由现有 Docker Hub 配置决定，以下 `YOUR_NAMESPACE` 需用实际值替换。生产稳定 digest 与 DB 版本本次未读取，不能用代码 head 或示例值替代。

使用下载的完整发布清单，先预览，再在你的测试环境执行：

```bash
python3 infra/release/switch_images.py --env-file /path/to/private.env --manifest /path/to/fact-only-release.json
python3 infra/release/switch_images.py --env-file /path/to/private.env --manifest /path/to/fact-only-release.json --migrate --apply
```

脚本通过 Compose 已有 `CORE_GO_IMAGE` / `WEB_IMAGE` 环境覆盖选择真实镜像；Core 同时覆盖 API、Worker、migrate/cutover。它 pull、按需运行迁移、以 `--no-build` 重建 Core/Worker/Web 并等待健康检查，输出实际容器镜像与 RepoDigests。已有 PostgreSQL/Redis/S3/Temporal 必须就绪；首次完整平台启动按 README 原步骤处理。不会输出完整 Compose 环境或秘密。

日常更新也可以明确选择可变测试渠道，但两标签可能在短暂晋升中指向不同版本，部署前以发布清单确认来源一致：

```bash
python3 infra/release/switch_images.py --env-file /path/to/private.env \
  --core-image YOUR_NAMESPACE/fluctlight-core-go:fact-only-latest \
  --web-image YOUR_NAMESPACE/fluctlight-web:fact-only-latest \
  --worker-build-id fact-only-ACTUAL_SOURCE_SHA --migrate --apply
```

测试环境使用独立 Temporal namespace 和独立数据库/对象数据。Worker 启动会切换当前 Worker Deployment 版本，使用不同 build ID 仍不能把共享 namespace 当作测试隔离。

## 回稳定

先将私聊模式改为 legacy、关闭视觉，停止创建新草稿；排空、取消或隔离本分支协议的等待任务，按 `fact-only-user-verification.md` 的回退事务关闭 waiting_reply/draft/continuation，再恢复旧版本兼容 settings 和模型绑定的私有备份。再明确切回两个稳定引用，省略 `--migrate`，避免旧迁移器面对新 additive schema 进行不必要的迁移：

```bash
python3 infra/release/switch_images.py --env-file /path/to/private.env \
  --core-image YOUR_NAMESPACE/fluctlight-core-go@sha256:ACTUAL_STABLE_DIGEST \
  --web-image YOUR_NAMESPACE/fluctlight-web@sha256:ACTUAL_STABLE_DIGEST \
  --worker-build-id ACTUAL_STABLE_BUILD_ID --apply
```

若按“重新构建 master 后更新镜像”的方式使用稳定 latest，将上面两个引用明确改成 `:latest`，并确认本次 master 的源 SHA、digest 和健康。**只重新构建 master，不会使仍指向 fact-only-latest 的部署自动回滚。** 已知稳定 digest 可直接回退，不依赖重新构建成功。

按用户最新验收分工，本次代理执行本地测试与构建检查；真实聊天、镜像部署、健康/私聊检查及回滚演练由用户在真实环境执行，未执行项不计为通过。
