# FO-D 媒体草稿回交与持久续接 技术设计

## 整体设计依据

依父任务 ../10-13-fluctlight-fact-only/design.md 执行；共同跨子任务合同不可被局部替换。

## 本子任务技术边界

持久draft与授权action refs分离，初次reply accepted needs_action且尚未发布；仅text/media_requests最小envelope，不把自然语言关键词当授权。Qwen调用现media.image.generate，经现CapturePlan/Kev/ComfyUI durable链，动作归draft并固定幂等；结果refs归属必须服务端核验。同步错误至多一次Cydonia调整；pending保存waiting。现media终态settlement通过稳定continuation intent启动受限原生Qwen run，携原draft/config/inbound身份恢复，不序列化Runner、不新增聊天轮次。draft-bound媒体publication owner统一，legacy媒体仍原链路，防media.go独立自动发图与reply再发双重发布。将asset-ready/组件提交/unknown分开；新version intent/queue fence避免稳定workerclaim，原superseded/cancel guard保留。

## 兼容与失败

T07/08/09/10：图片成功/失败/异步、附件发送失败重试、状态/物品约束、无sticker、取消及重复回交。T19：新draft continuation旧worker不会误消费，排空/隔离可回退。

## 证据

- ../10-13-fluctlight-fact-only/research/reply-runtime.md
- ../10-13-fluctlight-fact-only/research/vision-media.md


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
