# Goal native 提交仍缺失：现场与实施边界（2026-10-10）

用户继续反馈目标不完成。工作区干净，head43ec22b，专用 Tool 已提交并上线。只读浏览器检查同一用户已授权环境的模型诊断：13:07:25—13:07:49 request goal_review_event_146_533c5ba8368075db8f6ef201546bd71a 返回 {"summary":"正在评估 4 个目标。"}，response.tool_calls=[]；领域结果 retry/goal_evaluation_native_submission_missing，evaluated_goals=[]，submission_errors=[]，四个 unresolved_goals。Prompt 含最新版 native 指令与四目标冻结输入。旧 Stage 证据拒绝不是本次直接失败点，因为本次无任何 Tool 执行。

优先检验请求层三项：1. 执行阶段仍发送最终 summary JSON schema，模型在第一轮可合法提前终止；2. schema 是否真的装进物理 HTTP tools；3. tool_choice=auto 无持久化覆盖门禁，Provider忽略工具请求时如何保留失败。前两项用真实 OpenAI adapter + Eino Runner 的受控 HTTP 回放检测，不把强制 mock ToolCall 当作已证明实际 request 正确。

本地准确版本：Eino0.7.37/openai adapter0.1.13/ACL0.1.17。model.WithToolChoice 支持 per-request Forced/Allowed/Forbidden；WithRequestPayloadModifier可修改序列化JSON，签名含ctx/messages/rawBody。官方当前docs（Context7 /cloudwego/eino-ext）也确认该modifier用于Generate/Stream序列化请求修改。不要用WithExtraFields替换extra map以免丢enable_thinking/headers。

最小修复：typed Goal Evaluation 任务安装私有 request policy callback，每次物理请求读取DB accepted root coverage/active claim。仍有未接受root时：tools保持真实canonical schema、tool_choice=required、移除本轮response_format；预算和诊断按本轮实际格式计算。全部root覆盖后：解除required，恢复最终summary schema；允许已有可选object/plan行为，不把覆盖误当作禁止任何后续Tool。普通chat/Wake/Planner等未安装policy，行为不变。保持单Eino Runner、真实native/provider identity、数据库短事务、来源/CAS/paused和独立提交。禁止final DTO伪Tool、手工continuation、放宽成功标准或直接statuscompleted。

先写HTTP回归并运行RED：受控服务在有summary grammar或auto选择时返回现场summary-only；正确执行请求时才发真实native ToolCall，再按实际提交反馈更新coverage并返回最终summary。同一native loop覆盖首轮缺提交、拒绝不推进coverage、partial retry已有成功根、真正final轮schema恢复。Generate/Stream请求选项都需对应验证；DB生产coverage绑定在隔离PG测试中覆盖，无DB按约定SKIP。所有新代码由Trellis implement/check流程处理，本地交付，不部署、不写正式数据。
