# v6.2 复验修复记录

> 2026-10-24 复测集成：按用户要求，以提示词优化已提交版本 `4eed0858`
> 为基线，完整应用本批修复（集成代码提交 `7e94f7c8`）。提示词提交保留，
> 没有修改其文件。该版本推送至 `codex/fluctlight-functional-recovery` 供复测。
> 初始化保持单次生成；r3 报告新增问题将另行修复，不混同本批验收结论。

来源：用户提供的《摇光重做前端-复验v6.2.md》。用户已确认：收敛聊天等待界面、资料立即保存并后台编译、激活后的模型工作持久化执行；模型绑定与测试库备份保持现状；只采用真实发布成功的回复。

本轮在独立 worktree `codex/v62-retest-fixes` 实施，基于 `c11c121b`。另一 session 的提示词提交 `2d82c94e` 及其后续记录不合入本分支，不修改原工作区或原分支。现有无输入/输出 token 上限、原生工具与真实发布契约继续保留。

修改边界：前端会话级重试、发送受理与计时；初始化/资料/诊断/通知展示；后台编译作业、短事务资料受理、队列派发隔离与真实逻辑结果诊断。不重写提示词、模型语义、模型绑定或测试库数据。

验证结果见下表。受控浏览器与隔离数据库证据和真实模型验收分别记录。

## 22:40 报告更新

新基线 SHA-256：`eadf3ba7a0685559d84af2bda480e25c2b16ffc7f88b5cba58b7ce57e0472a73`。新增 Haiku 回归，强化跨模型的发送、派发、契约、保存与记忆问题；新增日记选错日期/终态无反馈。R11 已撤回滚轮故障，不再改抽屉滚动机制，只处理提示词展示可读性。空白激活关页已通过，描述激活仍须覆盖。模型切换、90–120 秒配置建议、用户发帖和认知正文兜底不自动纳入代码修改。

## 本批行为与追加需求

- 新发送不隐式重放旧失败消息；显式重试保留原 turn、幂等键和发送时间。未知发送结果仍要求先核对原身份。恢复记录按角色/会话保存；输入仅在请求受理或排队成功后清空，明确拒绝保留输入。
- 等待显示角色正在输入，30 秒后补充提示与诊断，90 秒后可停止，每次尝试单独计时。只展示实际保存的回复，后续失败不抹掉已经发布的消息。
- 创建原描述/JSON 保留并可编辑，显示具体错误字段。明确的激活结构拒绝返回输入，不复用被拒绝的分析；网络结果未知仍冻结原激活。草稿/命令迁移校验请求 ID，迟到结果不得关联另一草稿。
- 资料保存和激活先提交源资料与持久化编译 intent，已有编译器在事务外运行。编译失败可恢复；版本、退役、习惯、overlay、预算变化阻止旧画像发布。详情和初始化页展示实际后台状态。
- 按追加要求，初始化保持七项展示，编译纳入角色/世界确认项。实际任务依赖：世界绑定 → 人格编译 → 衣柜 → 参考图 → 设定图/视觉会话完成 → 显示头像投影 → 初始日程 → WakeUp。失败阻挡后续任务，状态查询可并行。
- 代码默认和本机已有 `llm.queue` 均为生成 1、向量 1。新增 `0101_serial_llm_queue` 升级将其他环境已保存的更高值收敛到 1，递增配置版本；head 重跑保留之后的用户调整。
- 配置行动类型使用真实多选枚举，通知按同一消息来源去重，诊断提示词的可读换行不改变实际请求。撤销过期滚轮故障的修复。

## 最终验证

| 检查 | 结果 |
| --- | --- |
| `pnpm generate` / `pnpm typecheck` | 全工作区通过 |
| `pnpm test` | Web 381 项通过；两个客户端测试通过 |
| `pnpm build` | 通过，生产资源不含模拟数据/fixture |
| 独立 `VITE_PRODUCT_MODE=mock` 构建 | 通过，仅演示构建包含模拟资源 |
| 新人格编译真实 SQL 回归 | 通过：受理/replay、保存后失败恢复、CAS、superseded、编译期间退役 |
| Workflow 包，含隔离 PG 完整派发候选查询 | 通过，世界绑定后 persona pending 仍挡住衣柜，完成后按依赖放行 |
| `0101`、Kev、旧版本配置升级回归 | 通过，一次迁移、版本 fencing、后续用户修改保留 |
| 生产构建 + 受控 HTTP 浏览器 | 格式错误 → 缺字段 → 修改描述 → 预览成功；控制台无 warn/error |
| 390px 同源 iframe 视口 | 可读单列表单截图，非移动设备实测 |

浏览器证据：[恢复后的预览](evidence/v62-retest/creation-recovered-preview.png)、[390px 预览](evidence/v62-retest/creation-preview-390px.png)。

SQL 使用本轮新建临时容器 `codex-v62-retest-pg`（tmpfs、25432），各测试创建/删除独立子库；没有修改其他容器数据。受控 HTTP 只证明生产渲染/恢复行为，不是用户真实模型的端到端验收。

## 基线失败与待复验项

Go `go test ./...` 仍有基线已存在的六项失败：

- `TestActorUserInitializationJSONPreservesOnlyQualifiedHumanBackground`
- `TestInitializationOwnerBackgroundKeepsStrictScalarTypes`
- `TestNoArgCatalogSchemaValidationAndCatalogParity`
- `TestPhase8CIUsesTheRealGateAndVectorEnabledDatabase`
- `TestBrowserRouteMatrixMatchesOpenAPIArtifact`
- `TestSurfaceRoutesMatchCoreAndMatrix`

后两项是既有 shared-experiences/overview 路由矩阵缺口。本轮 recovery 路由已登记，新增修改范围相关测试通过。本批不是整份报告全量验收：跨模型原生回复契约、多轮记忆/reflection、日记日期与无内容终态仍保留为后续真实复验项，未通过改提示词或伪造结果掩盖。

## 应用

部署时先执行项目现有显式数据库 migration 命令，升级到 `0101_serial_llm_queue`，再启动匹配版本的 API / Worker / Web。本次提交/推送不代表已部署到用户运行环境。
