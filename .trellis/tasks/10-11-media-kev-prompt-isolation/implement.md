# 执行计划

状态：已按用户批准实施，代码与自动化检查完成；真实服务/部署配置由用户测试，等待提交确认。

- [x] P0 创建任务，保存需求原文，核实入口与实际污染路径。
- [ ] P0 在实施前运行既有媒体/工作流/前端相关基线，记录 pass/skip/失败；安全备份实际本地 workflow/旧默认，核验运行配置。
- [x] P1 扩展媒体冻结契约/版本/显式约束来源；改变媒体能力 RequiredContext 与读取顺序，建立入镜范围及资料投影，覆盖无人/非本人/partial/unknown/多主体。
- [x] P2 兼容扩展 choice 多选与三个 media points，接入批量/跳过、全局/总/分项开关、原始审计及配置撤销；不改七点业务。
- [x] P3 改规范化隔离输入与中文输出，替换公共自拍/不适用current_capture渲染路径，最多一次修复，保留本人当前有效身份穿搭。
- [x] P4 提交前校验最终执行图与参考/LoRA/后置追加；提供最小中性workflow路由；处理缓存/版本/深复制及旧intent兼容。
- [x] P5 设置UI/本地操作入口同步，备份后直接启用本地媒体Kev；串联原意→决策→资料→LLM→Comfy→artifact诊断。
- [ ] P6 提供 M01–M40、不同中文表述和污染sentinel fixtures；独立Kev测试、规范化链路测试（真实生图必须显式开启）。
- [ ] P6 主代理完成A层测试与全范围质量检查；B/C/D服务可达时运行隔离测试，不真实发布；不可达记录未执行。
- [x] P7 更新受影响spec，写 media-kev-prompt-integration-report.md（真实命令/退出码/数量/样例/回退/限制）；按工作流提交复核与归档。

## 修改面
Core: tool_contract.go、builtin_capabilities.go、app_capability_context.go、current_capture.go、model_tasks.go、provider_context.go、media.go/media_quality.go、对应fixtures/tests。
Kev: internal/ai/decision、core/kev_decisions.go/kev_diagnostics.go、cmd/kev-config。
UI: KevSettings.vue、配置/诊断消费者；API变更同步browser contract与生成client（如果需要）。
高风险点：context resolver提前读取资料、ExecuteDirectTx状态版本校验、旧durable concept兼容、raw choice审计、最终Comfy正向图与参考节点。

## 验证命令
在 apps/core-go：
- go test ./internal/ai/decision
- go test ./internal/core -run 'Test(ReplaceMedia|SelectComfy|MediaRenderer|VisualIdentityReference|MediaComfy|CurrentCapture|Kev|ImageCapability|MediaPrompt|MediaCurrent)'
- go test ./internal/workflow -run TestKev
- 完成后按 backend quality guidelines 执行受影响包全量测试及 go vet。
前端：pnpm --filter @fluctlight/web test；pnpm --filter @fluctlight/web typecheck；pnpm --filter @fluctlight/web build。
真实验收新入口命令待实现后固定到报告；复用现有 Provider env，不包含密钥。DB fixtures 需要隔离 GO_CORE_TEST_DATABASE_URL；缺配置 skip 必须显式统计。
现有 LiveHandleTurn/WakeUp 只证明冻结/意图，不可冒充 Worker/Comfy 像素验收。

## 复核与回退门
实施前用户审阅本次prd/design/implement后才 start。各阶段自查确定性不变量，完成后独立只读核验线索由主代理复核并最终验证。配置备份后迁移，发现定制冲突不得无条件覆盖。测试失败不得把expected改成错误输出。

诊断契约全文超过自动注入大小；进入P5前主代理必须完整读取 `.trellis/spec/backend/fluctlight-diagnostics-contract.md`，不得将截断注入当作完整规范。

## 实施结果与用户接手项
- 已运行Go工作区用例/race、vet、工作区generate/typecheck/test/build；最终明细见交付报告。
- 固定场景及故障测试、独立探针入口已交付。P0运行数据库配置备份/启用和P6真实B/C/D验证按用户补充不执行，由用户在目标环境测试。
- 新增0060_media_kev，旧0056 SQL不改；补Worker非重试边界，防止规范化修复预算被Activity重置。
- 历史默认代码备份已保存，未改远程或部署配置。
- 未提交、未推送、未归档任务；真实验收与提交确认后继续收尾。
