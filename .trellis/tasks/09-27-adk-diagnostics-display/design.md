# 设计

保留每个 physical model run 一条记录。通过 bounded、Owner-only 的 DTO 增加该 run 所属逻辑 correlation/run、物理请求 sequence 和安全阶段摘要；若现有 `adk.model.input/output/tool.*` 事件已有足够的 model-run 对应关系，服务端在读取时关联，不在浏览器猜排序。没有 sequence 的历史行可按 queued_at/id 显示“顺序未知”，不能误标最终。

诊断视图按逻辑执行分组，组内逐轮显示 Prompt、对应 Response 与必要的 Tool 调用状态。明确区分 Provider response 包含 Tool call 的中间决策和最终用户可见回答；若最终 settlement 失败，显示真实状态。新字段经 Core Owner API、Browser BFF、browser-client 类型到 Vue 全链路映射。现有公开聊天 NDJSON 契约不变。技术诊断时间按 Owner 当前查看时区显示并标注；消息自身的历史发送时区由生活/时间子任务处理。

兼容旧 model-run row 与原有过滤/导出；新诊断字段不得包含 raw Tool arguments、reasoning、token-by-token response。
