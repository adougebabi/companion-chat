# 摇光正式系统可用性修复与功能补全

## Goal

对齐已确认 v3，修复真实运行故障，补齐 Actor 资料、世界 NPC 和核心功能闭环，并完成持久化与真实浏览器验证。

## Requirements

- R1：启动真实 Web/Core/PostgreSQL/必要依赖，从业务请求、日志与只读数据定位并修复公共故障。不重置数据库、不关闭鉴权、不吞错、不伪造默认/demo 数据。
- R2：“关于你”使用真实交流 Actor，复用 Actor context/修订/CAS，提供自然入口、阅读编辑保存取消、错误/冲突/来源与修订，并验证最新资料被上下文消费。
- R3：完成 NPC 定义管理与运行世界关联、地点/生活规律、关系和事件来源。定义创建不代表结识，浏览不移动角色，应用与草稿分离，归档有引用保护。
- R4：按 v3、Quasar、蓝图和原审计检查并补全其他已确认核心能力，覆盖子页/菜单/标签/抽屉，补实际生产与消费链。
- R5：复用现有 BrowserClient/领域服务/组件/composables，不复制状态；保留 Owner/CSRF、NDJSON/取消/重试/幂等、版本冲突、来源、角色隔离与有效状态边界。
- R6：维护同一功能台账，同步 migration-correspondence.md、backend-missing-apis.md、access-status.md，分别标记代码、自动化、真实接口、浏览器和真实模型证据。

## Acceptance Criteria

- [ ] 安全测试环境可用，新库初始化/旧结构升级保留历史；正常业务请求无非预期失败。
- [ ] 关于你保存查询/刷新/重新进入保持；冲突保留草稿；Actor/角色/会话隔离；上下文读取最新正确版本。
- [ ] NPC 正式 API/UI 创建编辑应用读回，跨页 ID 一致；创建无关系；实际受控接触有可追溯事件/关系/必要记忆；引用保护有效。
- [ ] 聊天流式/历史/取消/重试/附件，生活物品、目标日程记忆的真实读写和冲突有效；异步功能验证消费及结果。
- [ ] 日记/共同经历/通知有真实来源与生产链；创作预览不污染生活，发布应用处理版本；设置范围与实际生效一致。
- [ ] 深链刷新、前后退、切换角色、登出、窄屏与部分失败验证；合法空数据有使用入口；未执行测试不标通过。

## Notes

- 用户已同意创建总任务并连续推进规划、实施、验证。执行稿已完整读取：/Users/vinson/Downloads/fluctlight-v3-functional-recovery-and-completion-prompt.md。
- v3：/Users/vinson/Documents/文档/个人/ai/fluctlight/prototype-v3，固定 C 鼠尾草青绿；蓝图同目录 fluctlight-functional-blueprint-v1.0.md。
- 基线 master，53916647db2cb1e01b4d0f4131e19534eb0ffa7a。既有未跟踪审计任务和 docs/product/ 保留。
- 不做桌宠、Live2D、时间线回档；永久伤害继续禁用；不替换 Agent 框架/对话循环或改变 Qwen/Cydonia 分工；不在生产角色制造测试历史。
- 最新用户范围优先，旧清单分类不构成核心功能实施上限；蓝图明确 P2/P3 增强不自动扩张。
