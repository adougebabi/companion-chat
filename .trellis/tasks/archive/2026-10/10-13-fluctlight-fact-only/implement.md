# 执行计划与审阅门

用户已明确批准最终方案开始实施（2026-10-13）。

## 子任务顺序与验收归属

| 顺序 | 任务 | FO 范围 | 验收范围 |
| --- | --- | --- | --- |
| A | 10-13-fact-only-release | FO-00/01 | T18/19 基线、双镜像渠道、前置 grammar 修复 |
| B | 10-13-fact-only-config-context | FO-02/03、08预算 | T01/04/05/06/07/17 config/context |
| C | 10-13-fact-only-private-reply | FO-04/05 | T02/03/10/11/16/17 |
| D | 10-13-fact-only-media-handoff | FO-06 | T07/08/09/10/19 |
| E | 10-13-fact-only-optional-vision | FO-07 | T12/13/14/15/19 |
| F | 10-13-fact-only-acceptance | FO-08/09/10/11 | T20 + T01–T20 联合签收 |

## 实施门

- [x] 用户任务创建授权、原文保留、origin/master fetch、隔离分支和本地master保护。
- [x] 原生私聊、配置/上下文、视觉/媒体、发布基线研究落盘；高价值出处主代理抽查。
- [x] 复杂任务需求/设计/执行规划与真实spec/research上下文清单。
- [x] 用户确认最终方案；按顺序启动各子任务。
- [ ] A cherry-pick既有6e11490并检查与原origin基线区别；按功能独立commit，不改master。
- [ ] 每个子任务先读其完整prd/design/implement及适用spec，再按顺序实施/检查/记录证据。
- [ ] 用户规定子代理仅探索检索核验；主代理负责设计、产品代码与最终验证。原生能力与required阶段按适用上层指令推进，不擅自修改Trellis workflow/派生角色配置。

## 验证命令（实施后运行）

```bash
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go test -race ./...
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go vet ./...
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" go -C apps/core-go build ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
./infra/acceptance/check-core-openapi.sh
./infra/acceptance/check-compose-bind-sources.sh
git diff --check
```

定向测试先跑新增/受影响reply/context/provider queue/visual/media/HTTP cases，然后做完整联合gate；API生成一致性仅针对生成产物，不能用全工作区dirty视为失败或忽略真实diff。集成使用disposable PG及测试发送出口，经真实Registry、ExecuteTool、ADK回填与短事务；无DB时SKIP不可写成pass。取消/认知终态失败/并发重试/async completion/组件部分失败为必须失败注入。

真实模型按原文三条各10次聊天fixture走业务链路，不重跑direct基线；缺当前LM Studio/模型配置时不自动加载/换量化，列未执行与可执行命令。采样已验证值目前不在附件，通过既有配置读取/用户设置，不编造。

远端CI应推本功能分支触发，记录两镜像sourceSHA/tag/digest。没有gh或其他已授权connector时先确定可用读取方式；push/构建/部署状态分别记录。Docker daemon目前不可用，Compose健康/registry-image smoke/测试环境回滚未执行；不自动启动/替换真实环境。CI本地结构检查不等于Actions成功。

## 回滚点与交付物

- 每子任务独立功能提交，稳定master完整保留；无reset --hard/清理旧改动。
- 新模式置legacy + vision关闭 → 停新draft →排空/取消/隔离新协议任务 →恢复兼容settings私有备份 →明确切Core/Web stable refs → pull/up --no-build →digest与健康/私聊检查。
- master重建不自动改变仍指向fact-only-latest的部署；已知stable digest是优先即时回退依据。
- 交付至docs/：实施报告、三种配置例、所有视觉调用点清单、部署回滚手册、分层测试报告、一轮脱敏trace、Core/Web发布manifest。
- 最终只有各子任务验收和跨子任务兼容性/回滚检查完成才归档；环境未执行项不能标通过。


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
