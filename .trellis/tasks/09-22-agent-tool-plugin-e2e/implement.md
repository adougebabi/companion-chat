# 实施与验证顺序

状态：in_progress。用户已批准设计并实施。Agent/Tool 迁移和最终非生成式集成回归已收敛；真实双 E2E 等待用户串行实测，不能宣布整个任务完成。最新证据以 acceptance-matrix.md 和 research/runs 为准，原计划清单保留作为最终核对项。

## 0. 基线与环境（已完成静态盘点，真实连通待执行）

- [x] 保存 request.md、HEAD、52 项原有变更路径。
- [x] 明确 15 个注册业务 Tool、16 类完整模型任务与视觉完整任务目标；记录原生框架源码和问题位置。
- [x] 运行工具链/Docker 可用性检查并记录 exit code；仅检查配置项非空不输出值。
- [ ] 实施前逐一完整读取即将修改代码与相关规范；扩展负向搜索核实遗漏 prompt/业务能力，增补固定期望矩阵。
- [ ] 通过已有配置建立一次性依赖栈与测试账号，验证模型/媒体连通；安全记录 run 信息。配置路径显式传入，禁止误用默认共享栈。启动/清理复用 infra/compose/run-platform-smoke.sh、actor-chat runner 的隔离逻辑。

## 1. 一条真实链路验证结构（不是最终交付）

- [ ] 在现有 Agent/Capability 边界定义统一 Tool 执行命令、operation ID 和实际结果契约；短事务与领域服务独立装配。
- [ ] 以 memory_event → memory.recall + 正式认知 Agent 打通写入、读取、模型继续与真实结果关联。
- [ ] 删除两轮限制/写后提前停止/超限成功；保留 Provider queue、真实 call ID、取消与 final schema。
- [ ] 确认真实 Provider 请求看到实际工具结果，DB 观察到提交；有外部阻塞先完成受控正确性，真实行标 BLOCKED 不报完成。

## 2. 全部业务 Tool 迁移

- [x] 查询：memory.recall、relationship.lookup，最小装配与授权输入。
- [x] 状态：scene_event、presence_event、schedule.replan、affect_event、memory_event、active_memory_event、capability.request；不依赖 caller transaction/认知阶段。
- [x] 发布：conversation.reply、moment.publish，复用正式发布服务和 outbox，删除外层重复发布。
- [x] 媒体：media.image.generate、visual_identity.initialize 以及 canonical 保存等原有能力；真实受理、真实任务产物、幂等。
- [x] 人格：persona.takeover、persona.switch，实际领域提交与审计；不因 internal 标签排除。
- [x] 每项同步补独立成功/业务失败/产物/依赖异常/重复/适配执行测试。

## 3. 全部 Agent 及生产调用方

- [x] 全部 16 类 prompt 分别正式定义：init、main、wake-up、takeover judge/reply、persistent switch、native、daily review、reflection、summary、schedule generation/replan、media prompt/quality、visual vision/patch。
- [x] 视觉完整任务独立入口：生成→实际图片→观察/判断→保存；复用 durable checkpoint 不再靠另一 Main 补后半段。
- [x] API/对话/后台/worker 调用全部迁移；外层只提供输入和接收结果。
- [x] 流式、中间工具内容与最终结构化输出分离；运行和用户隔离；提交后错误保留事实并禁止重放。

## 4. 旧路径清理与规范同步

- [x] 删除自研 continuation、重复 dispatcher、延迟代执行、旧场景门禁、sidecar 执行兜底及 action-only 限额成功。
- [x] 替换保护旧实现的断言，逐项映射保留的产品契约，更新已有规范冲突条款。
- [x] 搜索所有 producer/consumer，确认无生产旧入口和双发布，检查用户原有变更未丢失。

## 5. 双 E2E 与有效历史回归

在现有 Go 测试体系与 infra/acceptance runner 内实现测试选择和结果汇总，不创建平行测试平台。计划提供以下入口（现已实现；live 套件按用户安排在最终代码上串行执行）：

- [ ] 媒体质量回归：首轮 retry/reject 均将结构化检查反馈带入 Media Prompt Agent 并生成第二张；二轮 pass/retry/reject 都核验第二张资产完成发布、实际 verdict 留痕且无第三次生图。受控决策/工作流测试已通过；PostgreSQL 反馈持久化测试因隔离库环境未配置而跳过。

```sh
# 扩展现有 runner 支持 suite/对象选择；每个命令都执行必需用例计数检查
infra/acceptance/run-go-live-provider-smoke.sh --suite tools --tool memory_event
infra/acceptance/run-go-live-provider-smoke.sh --suite agents --agent conversation_cognition
infra/acceptance/run-go-live-provider-smoke.sh --suite tools
infra/acceptance/run-go-live-provider-smoke.sh --suite agents
infra/acceptance/run-go-live-provider-smoke.sh --suite all
```

实现后记录最终实际参数，以脚本 help/测试列表为准；兼容现有普通 smoke 入口，但不得把其退出 0 当双 E2E 结果。

- [ ] Tool 全矩阵：每个正式边界，无 Main/伪 Agent；真实依赖成功，业务拒绝，独立查产物，依赖故障注入补充，重复，正式 Eino adapter。
- [ ] Agent 每项真实闭环：正式 prompt/context/provider/tool；按职责分配无工具、1轮、>=2轮工具/3次决策、失败继续、写后查、多调用、提交后模型失败、超时/取消/超限/非法输出、隔离。
- [ ] 随机秘密写库后从当前输入/预装上下文排除答案，通过正式 memory.recall 获取；记录下一模型请求及引用结果。
- [ ] 真实流式与最终完成，真实图片多模态 payload 及视觉判断，媒体异步真实完成产物。
- [ ] 两个隔离变异：断工具回填、假写成功，目标测试必须非零；撤销变异并核验无生产故障开关；最终代码双完整套件重跑。
- [ ] 全量必需行检验：缺项/0 tests/SKIP/BLOCKED/失败均非零；失败尝试也保留，不能挑偶然成功重跑。
- [ ] 所有受控模型测试明确归为 controlled regression；不能替代真实成功链。

已有验证命令（按改动范围执行；记录 command/exit/skip，不预先声称通过）：

```sh
go -C apps/core-go test ./internal/ai/agent ./internal/ai/model ./internal/ai/prompt
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test ./...
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go vet ./...
GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go build ./...
infra/acceptance/run-phase8-contract-gates.sh
infra/acceptance/run-go-live-provider-smoke.sh
FLUCTLIGHT_ENV_FILE=infra/compose/fluctlight.local.env infra/compose/run-platform-smoke.sh --clean
pnpm generate
pnpm typecheck
pnpm test
pnpm build
```

模型/数据库变量从既有安全配置装配，不把 secret 放日志。整仓测试按实际耗时与故障定位运行，DB skip 单独报告。跨浏览器契约修改时执行 browser race/vet/build 与生成契约检查。

## 6. 最终收敛

- [ ] 维护 acceptance-matrix.md：全部对象/用例/类型/状态/证据，每项链接 actual log 和产物。
- [ ] 最终受测版本 = commit + working-tree diff digest；记录依赖/配置变量名称，保留全部尝试及退出码。
- [ ] 主线程最终验证实现、生产接线、证据和未完成项；根据当前 Trellis 规则执行质量复核/规范同步/提交收尾。
- [ ] 只有双完整真实 E2E 与全部能力通过才标任务完成；外部阻塞时明确实现进度与验收状态，继续可独立工作。

## 风险文件与回退点

mutations.go、wakeup.go、turn_takeover.go 的提交与发布；capability_runtime/core.go 的身份/结果；eino_model_runtime.go 的队列与流；visual_identity.go 的 checkpoint/CAS；memory 生命周期与来源。每切片验证后再扩展，回退仅本任务精确 diff，不回滚整个脏工作树。服务临时栈完成后按唯一 project 清理；不得对共享服务执行 down -v。
