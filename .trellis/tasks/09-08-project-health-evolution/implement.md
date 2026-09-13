# A批执行手册

状态planning，当前只产出方案。执行按S顺序；并行仅用于独立探索，不让实现/验收子代理替代主代理。用户既有“基础优先/单次认知”无需重复确认。

## S00 开工前冻结基线

- 读取本目录prd/design/acceptance/roadmap及相关spec，不从聊天记忆猜范围。
- 刷新git status/diff与HEAD，记录其他任务正在改的文件；尤其provider_context/composer、wakeup/autonomy、schema、workflow_ops、capability。
- 每个旧发现按符号重查；别人已修好的用验收证明并保留，不能回退到HEAD重做。
- 选择明确的交付基线/分支或隔离工作区，不复制未完成的未知改动；与当前工作避免写冲突。
- 读取trellis-before-dev及backend provider/cognition/memory/diagnostics/workflow/persistence/error contracts、BFF/API、对应frontend指南。
- 确认A范围已审阅后task.py start；本轮不执行start。
输出：verification/environment.md与变更归属表。阻断：无法确定当前工作区归属时先解决边界，不覆盖。

## S01 先建立可失败的针对性测试

对应T01–T07/T14/T19/T20。
- 构造A/B人格和真实producer到HTTP payload夹具。
- 初始化前置错误、Provider非2xx、非JSON/schema失败夹具。
- 旧Redis通知丢失恢复的可控时钟+PG夹具。
- schema枚举互认、reflection RawMessage/appraisal/引用和隔离历史记忆夹具。
先记录当前失败，避免测试只复述准备写的实现。既有功能正常用例做回归对照。

## S02 A01 输入修复

依赖S01。按design第2节改最小producer/composer边界；当前profile权威、字段完整性、消息去重。
保留初始化/媒体特殊路由，旧后台realization只修输入，不增加新调用。
检查实际tools/schema未重复，reflection证据保持已有修复。
验证T01–T05；失败不得绕过必填校验去发模型。
回退点：只切回对应prompt版本/代码，不改Foundation事实。

## S03 A02 操作关联与错误

入口在配置前产生ID，贯穿BFF/Core/Provider；阶段映射和details同步客户端。
设置/网络/模型/领域失败分别有可查cause，页面可复制与跳转。
验证T06/T07/T23，并验证完全网络断开退化。
风险：固定code消费者与camel/snake边界。以生成源改契约，禁止手改生成客户端。

## S04 A03 attempt与有界sink

先分离执行ID分配与日志写库，再改attempt；确认Provider队列即使日志失败仍有非空ID并限制并发。
增量schema/nullable字段；每进程单sink，有界记录/字节/timeout，乱序终态保护，stderr不递归。
Provider transport/parse/schema/domain状态分别保留。
验证T08–T10/T23，故障下业务提交结果保持。
回退点：保留新增列，切兼容writer；不回填虚构历史attempt，不删日志以逃避冲突。

## S05 A04 页面直观诊断

先实现“失败阶段/有证据原因/影响/下一步”受控投影，再接失败位置的直接入口与现有诊断页；自动关联当前操作，不要求手工填ID。
按配置缺失、Provider拒绝、结构校验、超时未知根因、部分成功、自动重试/取消/被取代、诊断源不可用设计可复现UI夹具。具体文案形状见design第4节。
最新失败摘要先到，底层workflow历史按需展开和分页；领域影响据已提交事实，不能依模型文本判定完成。
改Core/BFF/generated contracts/前端loader，保留稳定错误code和旧字段；查询不扩成导出工程。
当前SDK文档只查询所需失败摘要/分页API；按AGENTS使用ctx7先library再docs，不发送秘密。文档/API不符先调整适配，不硬写假签名。
验证T11–T13/T23，并实际浏览页面：首屏能回答四个问题，至多一层展开看到依据；不需要下载、手填ID或反复重跑才能理解原因。
不修改导出接口，不新建LLM日志分析、监控大屏或批量恢复功能。

## S06 A05 唤醒deadline

先提交增量migration与回填/rollback脚本方案；后加事务deadline更新、due scanner、expiry只提示、Provider前与提交前guard。
状态/锁/版本按design表实现，next_attempt与quiet分离，旧run不重放。
测试T14–T18：Set失败、通知丢失、Redis/Worker重启、新聊天、双Worker、取消、旧库、保存历史。
上线切换需要支持新合同的Worker与旧expiry推进关闭；本批只完成可丢弃环境验证，不部署。
回退点：关新scan/对账run/兼容代码/按剩余deadline恢复旧TTL，不删事实、不重置cycle。

## S07 A06/A07 记忆合同与计量

共享visibility枚举，保留已修reflection解码与证据；记录memory source/revision、embedding、检索选入和最终prompt分项。
完整payload计messages/tools/schema及usage来源，不拿字符数当token。
为M01–M07生成基线入口/报告；A不实现B的大检索、自动revise或预算框架。
验证T19–T23，真实记忆对照使用可丢弃fixture；若合法候选到prompt断裂必须修，>200等按B明确未完成。
回退点：诊断新增字段兼容关闭，事实原样保留；不要批量重嵌入真实记忆。

## S07a A08 目标与意图最小闭环

依赖S02–S04的输入/关联与S06可靠唤醒，具体合同见goals-progression.md第2节。
1. 刷新当前源码，复现模型缺ref而写端要求ID、progress被忽略、过期仍pending、失败未反馈；按同名G/G2防错归属。
2. 实现可回解短ref/快照及同批create引用，更新schema/composer/metadata规则，保留owner/profile/CAS。
3. 实现已支持数值字段写回及before/after证据，complete与progress一致；过期结算及paused不可执行。
4. 冻结Action携带受控goal/intention来源，统一成功/终态失败结果事实与反思；重试中错误不重复推进。
5. Web按目标→意图→行动结果展示真实进展/阻塞/下一步，修active统计；旧无关联记录明确未知，不按名称补造。
6. 执行T24–T29；真实可丢弃端到端G/I1/I2闭环必须完成，不能仅SQL单测通过就宣称推进有效。
schema/归属审计增量迁移，保留旧行和已冻结Action兼容读取；本批不回填/自动修正真实历史、不实现B05新调度器。

## S08 契约、集成与交付

- A全部相关测试与迁移/历史replay完成，结果按ID列出通过/失败/跳过。
- A结果与B已知失败分开；真实模型未跑就明确，不用单元测试代替体验验证。
- 更新变更对应spec及README陈旧表述；不能为了对齐旧文档恢复两次聊天调用。
- 审查新增接口/配置/迁移/rollback及业务diff归属，主代理做最终验证。
- 输出变更、原因、测试、残留B项和风险；不部署/清理生产数据/自动推送。

## 验证命令（参考已存在项目命令，实施时按范围运行）

仓库根目录：
~~~
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go test ./internal/core ./internal/httpapi ./internal/workflow ./internal/migrations
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/gateway-go test ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
~~~
generate仅在契约变更时执行并审查diff；最后相关Go包race/vet和真实PG/Redis/Temporal集成按变更需要补齐。
当前redis_triggers_test.go在本地listener不可用时可skip，必须核对测试摘要；migration SQL contains不代替真实升级。
真实persona脚本为scripts/e2e/persona-layers-real.mjs，使用FLUCTLIGHT_E2E_BASE_URL/ORIGIN/PASSWORD等；仅安全读取注入凭据，不打印，不写文档。它有真实副作用，不直接指生产。
没有环境时报告缺哪层，不把mock通过提升为整栈验收。

## 后续任务划分

A暂不批量建子任务，按S作为可验证检查点；如果改动大小要求拆提交/子任务，按输入正确性、诊断、唤醒、记忆验证、目标最小推进独立交付并写依赖。
B必须等A的输入/计量与恢复基线；C等基础能力稳定。父子树不表达顺序，依赖要写进各自文档。
