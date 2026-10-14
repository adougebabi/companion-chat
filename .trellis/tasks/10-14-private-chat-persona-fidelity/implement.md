# 顺序实施与验证计划

状态：in_progress。用户已在最新完整方案后明确要求 PLEASE IMPLEMENT THIS PLAN，实施授权成立，task.py start 已执行。范围为一个整合任务的四个顺序切片，不在第一片完成后结束。

## 0. 开工门禁
- [x] 用户同意创建任务并进入规划。
- [x] 任务书完整副本及 source SHA256 保存，四项只读审计汇总。
- [x] 基线 HEAD / dirty文档识别；其他会话新增未识别文档继续保留，不纳入本次提交。
- [x] 既有受控基线执行，跳过原因分列。
- [x] 用户在最终规划摘要之后明确同意实施。
- [x] 激活任务，读 trellis-before-dev 和 relevant backend/frontend specs；主代理实现，子代理仅独立探索/核验。
- [x] 刷新其他会话工具参数/可用性改动的局部状态，保留原改动；冻结本次实际输入与source hash，不能把初始HEAD当干净工作区基线。

## 1. 基线与协议（T0/T2）
- [x] 在改业务代码前冻结温知意完整原人格、活动/时间/关系/Goal、原小说式历史/current input、model/settings/tool versions；材料边界 reconstructed/synthetic/missing 明确。
- [x] 用现有 final transport capture 生成 A 实际装配快照与 module metrics，不把显示转义当 raw wire。
- [x] `reply_context.go` / Prompt operation slot 仅 private作用域替换v5，第4节协议全部必要边界；Moment/legacy保持。
- [x] 对 final HTTP messages 断言 stable协议只一次，无旧“言语交织神态”追加；DB旧config fixture也覆盖。
- [x] 导出 B（仅协议）的脱敏实际请求 / input diff，可单独回放，不变模型采样。

## 2. 媒介、人格、状态（T1/T3/T4）
- [x] `fact_only_context_views.go` / scoped private entry 用真实 transport来源投影 channel和有证据空间关系，两处一致。
- [x] `writerTaskInput` /现有 persona helper 加完整portrait的只读解释视图、source/hash/profile/版本；不改变原数据/编译存储。
- [x] 原文回退严格限定授权生效profile与可用source，不把load mismatch变成另一人格。
- [x] 普通回复去无关身体细节，真实活动/时钟/Goal受保护；复用当前 `persona.detail` pure QUERY /实际工具结果保障外貌询问和语义跟进、媒体完整数据。
- [x] no source mutation / source hash unchanged / profile switch / media scopes回归，输出 C input diff和保留/省略理由。

## 3. 历史、发送前修复、闭环（T5/T6/T7）
- [x] 保留已有完整历史和真实role，引入历史原文/摘要/投影来源语义及不确定风险标签；无可靠标记默认全文。
- [x] `reply_provider.go` parse/validate seam 添加多信号文体诊断，作品/自称/引用/单句/不确定豁免。
- [x] 同模型无工具repair，传原候选/完整当前人格/权威事实/冻结媒体，禁止任何媒体/现实执行。
- [x] `fact_only_reply_capability.go` / `reply_media.go` 复用同draft JSON snapshot、CAS一次额度和候选 trace；总超时/cancel/lease/input budget/failure policy有效。
- [x] 首稿→原native image request→pending→真实结果→adjustment→repair→一次提交→final cognition组合受控回归。
- [x] 原 Qwen JSON修复只结算，receipt text=committed DB text；保留取消/新消息/故障恢复防迟发边界。
- [x] 导出 D input diff、首稿/diagnosis/repair/final/body/media/draft同一记录。

## 4. 版本、配置、回退、全量验收（T8/T9）
- [x] `reply_vision_settings.go` / `private_reply_run.go` / existing settings UI 增最小表达版本入口；新run缺省v5，旧draft缺字段按v4。
- [x] existing `binding_snapshot`保存Core-owned contract版本/hash/只读view/medium版本；重试/恢复/缓存/publish诊断不重新拿当前常量误标。
- [x] config/binding跨run重领一致，profile/actor隔离、facts刷新、旧prepared replay、新wait恢复/rollback R36组合。
- [x] 全请求仍走已有授权脱敏诊断；不新增永久完整聊天采集，candidate trace有draft范围及原retention。
- [x] 完成 acceptance-matrix R01—R36，每项给测试符号/执行类别/证据/缺口，不能只复制旧PASS。
- [ ] 真实模型回放 NOT_RUN（缺live配置）：A/B/C/D clean/dirty各3次及温知意9轮/3类人格入口已实现；24个受控回放已执行，不能替代真实输出。
- [x] 模型不可接/隔离DB不可用时保留明确NOT_RUN，交付可运行入口、前提和完整失败样本。
- [x] 审核全部改动与spec；仅在实际代码变化涉及协议、查询、持久化、配置入口时跑对应全范围检查。
- [x] 核验现有分支CI覆盖；远端构建未执行则标明本任务镜像不可用，不动稳定标签。
- [ ] 主代理完成最终验证并更新spec；提交另按项目门禁交审，不自动合并/部署。

## 验证命令与证据
现有基线（已执行，受控stub）:
```sh
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" \
  go -C apps/core-go test ./internal/core \
  -run 'Test(FactOnly|Reply|FrozenProviderAssignment|ConversationReplyPublishes|VisionDisabled)' -count=1 -json
```
实施后针对性新增测试将纳入同组，最终必要质量门禁：
```sh
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go test -race ./...
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go vet ./...
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go build ./...
pnpm typecheck
pnpm test
pnpm build
```
本地httptest需要沙箱外回环监听许可；该许可不授权真实模型或发布。PG integration仅使用可丢弃测试连接，未配置不能计通过。

真实Writer复用 `TestFormalAgentE2E/conversation_reply` 与 `independent_reply_live_test.go`，需要通用live模型和reply专用BASE_URL/MODEL/可选API_KEY。正式持久化组合复用live HandleTurn tests并要求disposable PG；添加A/B/C/D专用回放入口于现有测试层，不新增生产实验服务。此前用户环境分工见 `docs/fact-only-user-verification.md`，本轮未自动调用真实服务。

## 可回退切片与高风险落点
- 协议变化需B独立输入对照；视图变化需C；修复变化需D。
- 同draft JSON快照 / lease / publication /恢复只可局部增字段，避免无意覆盖frozen contract或repair quota。
- 生效profile/persona mismatch、旧protocol兼容、media等待后的规则变化最易串缓存；R13/R32/R36需组合测试。
- protocol回退保留fact_only，旧镜像回退沿现有drain/cancel步骤；任何实际rollback/部署由用户操作，不在本任务自动执行。

最终实现及层次化结果见delivery-report.md和acceptance-matrix.md。代码/自动化完成，真实服务验收与提交待审核；不会自动archive、合并或部署。
