# 执行与核验
- [x] GP00 基线三测试、入口映射（初始化、Owner、goal.decide、intention.decide、Reflection、Resolution）
- [x] GP01 一次初始导入、人格动态愿望脱离、历史保护
- [x] GP02 Actor版本编辑、关系事实来源、相关执行复核
- [x] GP03 统一容量、集合CAS/幂等原子提交
- [x] GP04 独立正式Agent、查询提交回读Tool
- [x] GP05 来源/历史语义去重、旧后继收敛
- [x] GP06 稳定排序/人工保护、执行选择
- [x] GP07 依赖环/作用域/实际启动门禁
- [x] GP08 合并触发/Worker恢复/租约
- [x] GP09 所有生产入口与上下文接线
- [x] GP10 正式API/生成客户端/目标与Actor管理
- [x] GP11 additive迁移、dry-run/重入、超额治理
- [x] GP12 分层测试与交付报告
检查：隔离 PostgreSQL 核心测试；go test/vet/build；pnpm generate/typecheck/test/build；原生loop脚本Provider与真实Provider分开；60矩阵和8业务E2E逐项记录未运行，不将SKIP算通过。

## 交付状态（2026-10-09）
以上勾选表示实施代码与列明验证已交付，不代表所有业务验收完成。任务保留 in_progress：真实 Provider未配置；60项矩阵仍有25 PARTIAL、3 NOT_VERIFIED_REAL_PROVIDER、1 NOT_RUN_FULL_SCENARIO。完整组合缺口逐项见 docs/verification/goal-planner/20261009/acceptance-matrix.csv。

最终验证：Go全仓1939通过事件（1804叶测试）、25跳过、0失败；Planner race 30叶测试通过；最后容量提示修复后HTTP两包179叶测试通过；pnpm 92项通过、0跳过；generate/typecheck/build/vet/路由清单/diff检查通过。真实PG/Redis/Temporal Worker重启独立实跑通过。生产数据库未执行迁移或模型回填。

## 运行频率修复（本地实现与验证已完成）
- [x] 红绿测试：满五自动Planner不调用；旧queued full不调用；空位completion能规划；Owner仍可建议。
- [x] 正式日程AcceptSchedule每天一次触发，replan/replay不重复。
- [x] 评估coverage修正反馈与契约失败停止重试；Provider临时失败仍有界重试且不写成功memo。
- [x] 不变语义/纯时钟/自身结果不新增模型；Actor相关复核和晚到事件/私有profile不丢。
- [x] 覆盖原完成链/容量/依赖/闭环回归；独立Trellis检查、spec、提交和日志。

线上部署/真实Provider修复后回归未执行；原GP完整验收缺口继续保留。独立检查获得定向测试与恢复修复，未收到整体验收签字，主线程已复验最终race与build。
