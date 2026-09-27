# 时区与生活事件状态一致性

## Goal

让摇光和用户的时间各有正确来源，并让日程、当前事件的场景和活动按同一个生命周期推进。

## Confirmed facts and decisions

- 摇光自己的日程、当前时间及她发送的消息，按显式设定时区；未设定则用初始化识别的时区。用户消息按用户发送当时的时区显示。
- 目前 LLM 初始化只从描述中提取时区，缺失时保留 `null`，运行期退回 `Asia/Shanghai`；blank 初始化固定上海（`apps/core-go/internal/core/app.go:1196,1493`、`apps/core-go/internal/core/life_context.go:91-105`）。前端没有自动捕获初始化设备时区。
- 目前消息只有 UTC instant，无发送时区快照；聊天页按查看时浏览器时区显示，旧消息会漂移（`apps/core-go/internal/migrations/runner.go:226`、`apps/web/src/views/ChatView.vue:146-159`）。
- 已有日程分支不返回 timezone，下一当地日 workflow 可能退化为固定 24 小时（`apps/core-go/internal/core/workflow_ops.go:775-776`、`apps/core-go/internal/workflow/workflow.go:498-519`）。
- 当前 Life Context 是 Event > Schedule，但活动 run 独立存在，Event 到期后活动可仍显示为 `in_progress/deferred`（`apps/core-go/internal/core/life_context.go:161-212`、`apps/core-go/internal/core/effective_life.go:239-265`）。
- 用户明确：事件到期且没有明确延期就结束；购物事件结束不等于购入物品，不能以购物为特例。

## Requirements

- R3.1：初始化时按显式设定 > 初始化识别 > 稳定且可见的最终默认值确定摇光 IANA 时区；已创建人格修改时区走现有 Foundation 权威路径。当地日界、Context current_time 和日程使用同一生效时区。
- R3.2：用户首次发送时捕获并保存当时的时区/UTC offset，重试复用该快照；摇光回复保存发送时生效的人格时区。历史消息在之后更改设备或人格时区后仍按发送快照显示。旧消息没有 provenance 时明确使用兼容 fallback，不伪造来源。
- R3.3：详情、聊天、日程、事件输入及诊断时间都显式说明采用哪个时区；无时区 `datetime-local` 的解析与目标人格时区一致。
- R4.1：通用事件开始时，其 activity/scene/location 和活动 run 采用一致的有效期；事件到期且无明确延期，当前状态回到当时日程，活动 run 不再被投影为“当前”。
- R4.2：明确延期可延长事件和活动有效期；模型/Worker 迟到或失败也不能把过期事件当作当前事实。
- R4.3：活动结束与业务成功分离。购物活动可无购入物，只有已确认 acquired_item 才写衣橱；剪发、染发等效果同理遵守已确认结果。

## Acceptance criteria

- [ ] 固定 UTC instant 下，显式/初始化识别的两种摇光时区都能一致驱动 Context、日程选择和当地午夜 timer；已有日程分支不退化为 24h。
- [ ] 同一条用户消息在发送后更改设备时区、同一条摇光消息在调整人格时区后，显示钟点不漂移；首次流、重试、刷新、历史分页一致。
- [ ] 用户与摇光消息的时间显示带正确时区；跨时区创建事件不会因为 `datetime-local` 被设备时区解释而偏移。
- [ ] 通用事件有效时场景/活动一致；到期且未延期时，详情、Provider Context 与对话都回到下一日程。run 有可追溯终态，Worker 暂停期间也不冒充当前。
- [ ] 购物事件结束但未购入物时衣橱不变；确认购入时才写衣橱，且其结果不会让旧场景重新变成当前。

## Out of scope

- 不按职业/时钟硬编码“应该在哪个场景”，不把日程计划当作已发生的活动结果。
- 不回填旧消息原始发送地；UTC instant 无法反推出该信息。
