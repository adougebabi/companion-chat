# 执行计划

- [ ] 快照当前未提交改动，并逐文件确认消息、初始化、日程、事件、活动 run 和前端时间的现有路径。
- [ ] 修复当前日日程 ready 分支的 timezone 回传及 workflow 对缺失 timezone 的显式处理；以非 UTC 与 DST 日界测试。
- [ ] 加消息时区/offset 的向后兼容迁移、Core DTO/写入/幂等/历史、BFF 流式与历史映射、browser-client 生成源和产物。
- [ ] 首次用户发送捕获时区快照并贯穿离线/重试；摇光提交回复时冻结生效人格时区；更新聊天、侧栏、详情、日程和诊断时间格式化，覆盖旧行 fallback。
- [ ] 初始化缺时区时引入已校验的初始化识别/设备时区回退；保留明确设定优先级及 Foundation 修改后的 schedule invalidation。
- [ ] 修复 `datetime-local` 的目标人格时区解释，验证跨时区输入不偏移。
- [ ] 将活动开始、scene Event 与有效期关联；到期/延期形成通用闭环和 durable 收敛，失效 run 不进入当前投影。
- [ ] 让活动结束可无业务效果；只有明确成功结果才写衣橱/外观，测试静默跨日程与 Worker 迟到。
- [ ] 运行迁移、Go Core/Workflow/BFF、browser-client、Vue 测试与构建；最后做固定 UTC instant 的跨层验收。
