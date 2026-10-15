# Kev三工具基线与回复引用恢复

## 授权与来源
用户当前粘贴请求明确：Tool提供经过Kev判断，常驻仅conversation.reply、查询全部工具、moment.publish；失败也不能给38个工具。沿用本会话既有任务创建/开始处理授权。之前用户提供reply_evidence_not_authorized后的重试记录，确认raw历史消息source_ref和正式事实ref混淆；仅保存合成格式，不保存真实聊天或模型reasoning。既有feature branch push/测试镜像构建授权持续；不部署或回归真实环境。

## 要求
- 会话、WakeUp等公共工具运行的首个physical请求最多三项基线，基线与本运行授权安装相交。不自动安装foreign工具。独立Writer/tool-free任务保持零工具，专用Goal/VisualIdentity固定内部合同保持。
- 目录入口合并列表与点名按需加载，移除常驻capability.discover的第四项；其余工具只有明确查询并经Kev准入后进入后续native schema。完整执行器注册保持，实际HTTP/budget/diagnostics/执行许可用同一可见集合。
- Kev关闭、无配置、timeout/invalid/audit失败、版本变更都不回全量。显式No/不确定/不可用保持隐藏并给真实原因；取消传播。
- 目录返回授权目的/参数，不因此加载所有工具。完整请求先验证归属，不能部分加载foreign或绕过Kev。
- 媒体needs_action若对应工具准入不可用，Writer能消费真实unavailable并调整；不永久卡在等待，不伪造动作。
- evidence_refs可省略；历史message:message_…定位标识不能充当正式事实引用。无效/过时引用在Writer I/O前拒绝，模型改正后同run可继续；真实生成失败、并发lease、幂等/source/取消边界保留。

## 验收
受控Generate/Stream首轮精确基线；已查询且Kev yes的工具在后续请求可见并真实执行；失败/禁用/撤销最多基线且不可偷调用；跨run registry/schema不漂移；目录不全量披露native。坏历史引用→空/合法引用后同run成功一次、Writer调用一次；外来引用拒绝；真实模型失败不无限重试。仅本地/CI一次性PG及受控HTTP，不执行用户真实环境回归。

## 用户追加：独立回复上下文精简
真实环境反馈Writer输入超过2万字符，服务端prompt_tokens=13428。要求沿用认知侧上下文处理而非所有聊天历史一次性发出。复用已存在近期history/summary/working-memory裁剪，保留有效人格、当前输入/明确拒绝、Goal及权威状态。记录裁剪前后字符/估算tokens，处理本地估算与中文实际tokens偏差；不增加每轮摘要模型或Qwen社交指令，不改真实环境。

用户追加统一验收：Writer输出连续空行/字面转义换行归一化（代码区保留），提供固定私有原生 reply.image.request 入口，真实请求转现有draft-owned媒体执行，不用文字或SVG/path占位冒充图片。媒体结果调整阶段零工具，native请求最多2个，保持权限/Kev/source/异步结果与实际附件发布栅栏。此请求覆盖先前Writer始终零工具的约定；非图片Writer/调整仍零工具。
