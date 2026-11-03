# FO-A 分支构建隔离与回滚基线 技术设计

## 整体设计依据

依父任务 ../10-13-fluctlight-fact-only/design.md 执行；共同跨子任务合同不可被局部替换。

## 本子任务技术边界

从origin/master起点保留本地master，实施时cherry-pick既有6e11490工具选择/final grammar修复。改 .github/workflows/fluctlight-ci.yml 和小型infra发布/镜像切换helper，沿用Docker Hub认证、Dockerfiles及当前架构。唯一tag包含sha/run/attempt；matrix完成后收集两digest与source labels，串行promotion前查询远端HEAD。PR仅验证，无pull_request_target。测试namespace与数据独立，不能仅换worker build ID。master现workflow_dispatch不会发布；已知稳定digest/已有push rerun是即时路径，未获master修改授权不合入CI引导或业务。

## 兼容与失败

T18：source SHA正确、旧run不能覆盖新alias、PR不发布、两镜像全集合成功才晋升。T19：可按真实稳定ref回退，配置及新worker协议有隔离/排空步骤，未执行项独立标记。

## 证据

- ../10-13-fluctlight-fact-only/research/release-baseline.md
- ../10-13-fluctlight-fact-only/research/external-docs.md


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
