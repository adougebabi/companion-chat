# 旧库升级时整理人格画像

本步骤用于升级后日志出现 `working_persona_not_ready`、提示
`persona-backfill --owner ... --all --apply` 的旧库。
新建角色使用已登记的后台编译任务，其等待或失败状态可在页面查看和恢复。

维护前保留数据库备份，并暂停 Core / Worker。PostgreSQL、对象存储及模型连接应保持可用。
先完成项目正常的数据库迁移，再用与本次发布相同的 Core 镜像检查画像。
下面的 `OWNER_ACTOR_ID` 使用启动日志中 `--owner` 后的实际值。

先预览受影响项目，预览不会调用模型或写入画像：

```bash
docker compose --env-file infra/compose/fluctlight.env \
  -f infra/compose/fluctlight.compose.yml run --rm --no-deps core \
  /usr/local/bin/fluctlight-persona-backfill-go --owner OWNER_ACTOR_ID --all
```

核对输出中的 missing/stale 项后，执行正式整理：

```bash
docker compose --env-file infra/compose/fluctlight.env \
  -f infra/compose/fluctlight.compose.yml run --rm --no-deps core \
  /usr/local/bin/fluctlight-persona-backfill-go --owner OWNER_ACTOR_ID --all --apply
```

此过程调用现有模型，只发布源版本仍匹配的画像；不会切换当前人格或改写经历。
某一项失败时，输出保留其失败原因。修正连接或源资料后再次运行，已就绪项会跳过。
也可用 `--fluctlights ID1,ID2` 替代 `--all`，限定实例范围。
完成后启动匹配版本的 Core / Worker / Web，并核对健康状态与日志。
