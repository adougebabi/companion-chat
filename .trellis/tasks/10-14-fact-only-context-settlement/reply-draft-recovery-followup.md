# F20 当前轮回复草稿指针恢复

用户继续报告conversation.reply失败reply_draft_scope_invalid。继续当前已获实施授权任务，保留F19未提交变更和全部未识别文件，不重开规划。

## 已确认行为缺口

fact_only_reply_capability.go的factOnlyReplyService.prepare先由privateReplyLogicalKey计算当前source/run/主体/会话草稿身份，却在可选模型draft_ref不等于这个身份时直接返回ErrUnauthorized。此路径未查别的草稿，也未尝试当前轮cached/prepared/published恢复。仅有错误码，没有原始事故参数，不能断言真实模型用了哪一种旧值。

## 范围与验收

- 在Fact-only私聊边界把draft_ref视为恢复提示，实际草稿权威固定由当前调用source/run/主体/会话决定。错误、旧轮、foreign或无法解析的可选指针只能忽略/告警，不读取或复用其正文/媒体/配置。
- canonical当前轮prepared/published重放优先，媒体等待/就绪恢复继续原同草稿路径；存在误填指针不能导致多一次Writer或多条发布。
- 原生codec仅放行可选reply draft提示到服务端reconciliation；action_result_refs/媒体目标等真实效果资源保持原有验证。不得借本轮修复放开其他工具草稿授权或Moments发布范围。
- safe warning+diagnostic记录当前调用关联/原因计数，不泄露foreign ref或草稿正文。正常返回实际已提交结果；不能只把失败改handled而搁置本可继续的回复。
- source关闭/当前权限/取消/真实Provider和DB故障仍保真，错误指针不绕过这些条件。不改表达v5、一次style repair额度、冻结binding、publication去重、source逻辑身份。
- 回归至少覆盖：无当前草稿时错误指针仍独立生成；同轮缓存/已发布重放无额外Writer/发布；native未知可选alias可到恢复层、action alias仍严格；foreign/旧草稿无读写/复用；当前媒体恢复结果只从canonical draft取；失败/取消仍无假效果。

预计源码范围：fact_only_reply_capability.go、provider_ref_codec.go及最小专门测试；必要的可选结果warning字段/schema同步在相同reply契约内处理。若发现可信continuation source逻辑身份本身不稳定，先提供证据，不靠忽略指针改逻辑key。

验证继续纯/脚本native/临时PG分层，不连接已有数据库、不用真实模型、不操作生产workflow、不自动push/deploy/归档。当前PG临时实例已清理；需要PG时主线程会创建本轮独占环境。
