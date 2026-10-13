# 当前实施状态（2026-10-13）

用户已批准整个方案，最新补充：代理只做本地用例/类型/构建，不执行真实环境回归、真实模型长聊、部署回滚演练，用户负责。无需再问规划确认。

分支 codex/fact-only-reply-vision，master保留6e11490、origin/master e36b3be。提交：1c7ead6（带入既有grammar修复）、dd89271（FO-A构建隔离/脚本/任务全量）、ea6d37e（FO-B独立配置/角色/Writer context/UI）。没有push、部署或merge。

FO-A本地3个Python发布用例/YAML解析/py_compile/Go ToolFinalSchemaSeparation通过；FO-B定向配置、snapshot、context预算、22项Agent目录及Web vue-tsc通过。

当前active FO-C private-reply，代码未提交且还在接线/验证，不能宣告FO-C完成。已新增：capability_run_scope.go immutable registry view；private_reply_run.go按direct conversation映射固定模式/assignment；reply_provider.go tool-free正式Agent及父策略隔离/finish检查；fact_only_reply_capability.go事务外生成、DB草稿缓存/lease、现Publisher+ledger提交；provider_capacity.go复用队列增加writer子容量；Main专用fact-only协议；Wake走同一reply注册视图；0061 additive draft/消息metadata迁移。

重要待修/待核验：
- 最近日志模型测试首次因为sandbox不能bind；改sandbox外。NestedReply第一轮fail是旧fakeProviderNativeToolCalls丢弃空args，已改真实wire arguments="{}"，新测试模拟invoker参数顺序也已修正；最新定向race用例通过。
- ReplyDraft已新增LifeRevision/FactsRevision字段，但还未在generate/cache/commit接线，必须防缓存正文被重新锚定到新状态后发送。
- config retry_limit/legacy_fallback字段尚未完整实现运行失败策略；ctx取消已传递，DB generationErr cleanup当前WithoutCancel无timeout，需加bounded timeout。
- FactOnly capability媒体requests当前返回reply_media_action_required，FO-D还未实现needs_action/真实动作/异步恢复/附件部分重试。
- FO-E视觉gate/独立physical图片路由/初始化awaiting_review/显式测试按钮未开始。
- FO-F新Writer Agent live清单/诊断隐私/本地完整build、docs/config/trace/用户回归步骤未完成。
- 普通私聊通过现ExecuteTool/Pub提交；现user消息入口actualActor表direct mapping用于范围判断。新scope函数不得在未授权资源下调用模型。

本机无gh、Docker daemon不可用，无initdb/psql/Postgres.app可见；不会自动安装或启动真实环境。PG集成可写成opt-in，未跑必须报告SKIP。测试监听临时localhost需要require_escalated，锁定Go依赖缓存已下载在repo .gomodcache/.gocache。

继续顺序：修FO-C验证问题和缓存fence→普通闭环本地验证/commit→FO-D媒体持久续接→FO-E视觉可选化→FO-F本地用例/构建/docs/commit/用户交付。不要停在FO-A/B或把mock当真实回归。

FO-C普通文本/失败策略定向race测试已通过，准备独立commit。后续D设计选独立continuation表/独立Temporal队列与dispatcher隔离旧worker；必须同步waiting_reply的claim/replay/supersede/cancel/observer/final settle，不能改correlation绕过source fence。详细源码位置见research/media-resume-source-seams.md。
