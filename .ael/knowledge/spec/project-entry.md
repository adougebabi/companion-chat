---
{
  "id": "lac-project-entry",
  "title": "Fluctlight 项目工程入口",
  "kind": "spec",
  "status": "current",
  "paths": [],
  "source": "AGENTS.md; CONTEXT.md; reviewed migration proposal",
  "critical": true
}
---
# Fluctlight 工程入口

此文是入口与来源索引，不声明业务实现已通过验收。

- 活动工程流程仅为 AEL。旧 `.trellis/` 已完整归档到仓库外并移出工作树；历史出处路径通过 `.ael/imports/legacy-retirement.json` 定位，不继续运行旧 remember/update-spec/session-start 工作流，也不双写旧 journal。
- 项目概念和现行用户规则以 [CONTEXT.md](../../../CONTEXT.md) 与 [AGENTS.md](../../../AGENTS.md) 为准；AEL 的任务/租约不进入业务领域模型。
- 禁止向 Git 提交图片，含既有图片的修改。截图、日志、运行库与备份置于仓库外；不跳过图片提交钩子。
- 产品前端以已确认的 React/shadcn Radix 规范为准，不根据 README 中的旧 Vue/Pinia 描述恢复旧框架。模型输入不得恢复应用层 token 上限；时间超时和领域查询数量限制与 token 限制分别处理。
- 规范正文按任务查询与展开，不把全部历史或全部规范放入初始上下文。研究、旧报告和原始日志不是当前代码/验收事实。
- 本次已按用户要求删除仓库内 Go 缓存和 Python 虚拟环境。后续检查使用仓库外/Go默认缓存，不把旧 README 的仓库内缓存示例当成必须配置。
- 所有真实发布、业务数据库变更和基础设施操作需要相应任务授权；工程工作流切换不授予业务发布权限。

## 按需入口

- [后端合同索引](legacy-07e9eea9eac442a10f9d34f1.md)
- [前端合同索引](legacy-722dd21dcc7d29b46323c4a8.md)
- [React 技术栈与组件归属](legacy-b98cb585876a94318684c053.md)
- [模型输入构造（修改模型调用前必读）](legacy-9b89c41b4be449b963dcba95.md)
- [初始化与准备（原模块索引漏链，单列入口）](legacy-cb41a9af66866f54759af7f1.md)

## 待确认的局部合同

[Autonomy 合同](legacy-68be5396ba3fecfcf55a2f9f.md)已标记 conflict：缺少直接 Conversation 时，旧章节有拒绝/不创建与确保会话/可创建的相反表述。涉及对应 WakeUp/Goal/Intention 实现时，先核对事实与授权再解决；不自动挑选一段作为新的执行权限。明确的 prompt-unlimited 规则不因此失效。

3份旧阶段/旧记忆层指南归历史，其他原有合同的继承不等于全量业务语义重审；有明确旧宿主残句的文档附带来源可追溯的审阅说明。
