# 诊断任务详情：LLM Request / Response 修复

2026-10-30，承接 r19 和原全站任务。用户明确此处的主体应是关联模型调用的请求、响应和调用错误，而不是工作流时间、设施阶段、类型和整个任务 JSON。本批优先落实这一内容要求，不执行真实模型故障排查。

## 修改后的界面与数据

- 任务详情使用 Demo 的 860px 宽弹窗，只有一个实际任务标题。生产 `title=wardrobe.initialize` 会显示角色名及“初始穿搭准备”。
- 默认打开“执行记录”，直接呈现 `model-runs` 中存储的 Request、Response、`error_code` 和 `error`。桌面并排，390px 上下排列，长内容可滚动，无截断提交数据。原始 messages、工具调用、未知字段、字符串、false、0、null 和空结构均保留。
- “刷新调用记录”只重新读取现有数据。分页失败保留已确认内容；重试使用原 correlationId 与原 cursor。无关联标识时没有全局调用查询。空结果、缺 Request、缺 Response、读取失败分别说明；缺记录不等于从未调用。
- 通用任务失败提示置于“执行阶段”；工作流设施事件和完整任务 JSON 位于“任务信息”的次级技术披露，不再代替模型响应。状态、关联任务和原业务恢复仍保留。
- 衣柜阶段采用真实的 `preparation.result.inventory_committed` 和最终穿着状态，要求 task/fid 完全一致。dispatcher、workflow_start、activity 等生命周期设施名不再成为“已记录环节”。无匹配阶段来源时不编造进度和时间。
- `WorkflowRecovery` 未修改；原 allowed_actions、原 body/key/CAS、未知状态与已接受后的恢复限制不变。初始化仍可从执行阶段去配置并回到原流程，不重复激活。

## 验证与证据

- [最终模型调用浏览器索引](evidence/task-llm/index.json)：7 类 × 三视口 = **21/21**。包含完整内容、加载、读取失败与恢复、分页失败原游标重试、缺请求/响应、空记录、缺关联；检查精确关联、零写入、长内容滚动、860px/移动布局、Escape 和原入口焦点。
- [初始化返回复验](evidence/r19/task-llm-return-after/report.json)：**3/3**，保留原配置返回和唯一激活。
- [检查结果](evidence/task-llm/checks/results.json)：Web **748/748**、0 失败/跳过；定向检查 **38/38**，类型/lint、production build、383 模块 React-only 和 fixture 排除通过。
- 两个旧测试原先在默认视图查找通用任务错误，已改为先确认默认模型调用视图，再到执行阶段核对原错误；通知已读和原恢复请求的断言完整保留。早期失败日志保存，不计入最终通过。
- 主线程人工查看桌面/手机默认内容、手机缺失状态、长 Request 滚动后 Response 及读取恢复截图。所有图片在 `/private/tmp/fluctlight-demo-audit-20261028/task-llm-shots/`，不提交到 Git；路径和哈希见检查目录 `visual-review.json`。用户两张原截图保持原路径。
- Demo 48 文件、无关旧任务 108 文件哈希一致，见 `preservation.json`。

## 边界

Request/Response 来自现有诊断接口已经保存的内容；该接口原有的数据处理策略不变，不能把它称作包含所有 HTTP 头的原始网络抓包。前端不会补造历史上未保存的响应。

同一真实 correlation 可能包含多次执行和子工具调用，页面明确该关联范围，不仅凭时间或标题猜测某次 workflow attempt。读取沿现有 Owner 权限和关联 API，不改后端、模型服务、提示词、真实任务状态或业务数据。

本批 UI 证据使用实际生产构建与受控 HTTP，不冒充实时 Core/模型验收。原全站矩阵只追加具体观察，不批量提升；原任务保持 in_progress。
