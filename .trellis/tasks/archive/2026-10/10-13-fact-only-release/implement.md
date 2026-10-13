# FO-A 分支构建隔离与回滚基线 执行计划

父任务最终方案已获用户明确批准，可按顺序 start 本子任务。

依赖：先实施；其他子任务沿用本分支与发布基线。

## 顺序

- [x] 已带入6e11490为1c7ead6；master不变。TestToolFinalSchemaSeparation定向Go测试通过。
- [x] 补功能分支push与验证门；原actions/Dockerfiles/权限边界沿用。
- [x] 已实现不可变tags/全矩阵manifest/HEAD与digest门；3个本地发布用例通过。
- [x] 已添加默认preview的switch_images.py与docs/fact-only-deployment.md；真实镜像切换由用户执行。
- [ ] 本地结构与脚本检查、推功能分支后查真实CI/manifest（可访问时）；没有远端权限明确未执行。

## 检查与交付门

按父 implement.md 的命令与本范围定向验证执行，新增DB集成通过真实registry/ADK/事务路径；SKIP与未执行单独记录。完成本子任务后独立功能commit，更新spec及父验收矩阵，父联合验收前不宣告整体可用。任何新增配置/状态保持additive；回滚点遵循父design的stable镜像ref、配置恢复与新协议任务隔离。


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。

### 本地验证证据

`python3 -m unittest discover -s infra/release -p test_*.py -v`：3/3通过；Ruby YAML读取CI：通过；Python py_compile：通过；git diff --check：通过。远端CI/镜像发布与环境回滚未执行，由用户真实环境验收。Go定向用例第一次因sandbox DNS失败，已在sandbox外下载锁定依赖重跑。

Go依赖下载后定向测试通过：`go -C apps/core-go test ./internal/core -run ^TestToolFinalSchemaSeparation -count=1`。未执行全量回归、CI发布与环境部署。
