# Fluctlight 目标闭环实施报告（进行中）

基线：2026-10-07 master / ee1e468，初始工作区干净；实施分支 codex/fluctlight-goal-closed-loop。需求完整快照在 .trellis/tasks/10-07-fluctlight-goal-closed-loop/source-requirements.md。

## 状态
| 范围 | 当前状态 |
| --- | --- |
| G00 | 实际入口映射与A故障复现已执行；后续生产链继续核实 |
| G01–G03 | 执行资格/Attempt/标准基础已落地并分层回归；完整评价接B |
| G04–G07 | 待实施 |
| G08–G10 | 待实施 |
| G11 | 0050 migration已在一次性PG验证upgrade/rerun/rollback；完整存量修复待D |
| G12 | A范围42叶子PASS（41含race+1媒体authority恢复），0FAIL/SKIP；非全矩阵 |
| G13 | 本报告持续更新；六条最终闭环门禁当前未全部满足 |

## A 已落地链路
认领due fact先写Attempt，再装配projection/请求Provider；Tool事务读取live Goal/Intention、版本/身份/时间/权限，生命周期与新副作用串行协调。Provider失败、同步ACTION、QUERY/计划/no_op、async acceptance分别结算；真实外部operation ID等待结果并持有durable result workflow。Worker核对真实活动/媒体/视觉权威，未知六次后停止自动核对并阻止新执行，不盲目购买/发送。
标准 IDs 与 criteria_version 持久化；混合标准编辑/完成拒绝，旧版本评估CAS拒绝；标准编辑使旧进度失效，重复动作不按固定点数增长。Goal暂停/终结处置queued intentions；恢复重查expiration；硬期限关闭unstarted intentions。物理晚到结果与held logical状态分开。

## 环境与证据
一次性 pgvector/pgvector:pg16 容器、随机localhost端口、tmpfs，仅自有测试集群；秘密URL保存于/tmp任务运行目录，不写报告。使用随机子数据库；没有访问/迁移生产库。OrbStack启动后Docker可用。
原始命令及json日志见任务research/a-*.jsonl与a-checkpoint.md。DB已执行empty/head/0049模拟upgrade/head rerun/malformed rollback。42叶子测试为领域/隔离PostgreSQL/native scripted Provider/Temporal workflow unit evidence；不是live Provider或真实媒体质量。
最初广泛回归失败中14项于原始ee1e468同样失败（Tool清单、摘要语义、context预算等），保留baseline-failure-comparison.jsonl。全范围门禁仍待后续重跑，SKIP不当PASS。

## 部署与风险
0050新增Intention retry/hold字段、Attempt lease/wait/reconciliation字段、Goal criterion_ids/criteria_version/deadline_policy；旧状态/证据/历史保留，不生成历史阶段或完成证据。生产先备份并用隔离副本验证迁移，API/Worker只verify schema，明确运行migration CLI；本轮不自动修改用户生产数据。新schema写入后优先停Worker并前向修复；旧二进制回退需要停写与备份恢复，不能以删除新字段冒充无损rollback。
当前未验证全Worker部署、完整无Intention聊天完成链、Review/Owner UI、真实模型与媒体。B/C/D继续实施后补证据ID、全矩阵及请求预算变化。
