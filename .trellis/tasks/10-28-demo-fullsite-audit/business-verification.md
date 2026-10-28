# 业务接入结论与证据边界

本轮主线为UI/UX比对与修复。真实后端核查用于判断界面操作是否接入已有业务；不以扩展后端回归替代页面完成。2026-10-29后续修复F41–F59均为前端或验收材料，没有修改后端规则、LLM提示词或token政策。

## 当前真实环境

本任务隔离Compose为`fluctlight-demo-audit-20261028`，Web/API为`http://127.0.0.1:18480`，使用本任务Owner与独立PG/Redis/Temporal/MinIO。初始发现文档`research/real-environment.md`描述的是启动前历史状态，不代表当前环境仍停止。后续只同步前端静态构建并保留runtime-config，没有重启原Core/Worker/数据库。

## 已执行的真实链路

|范围|实际证据|能够确认的结论|不能扩张的结论|
|---|---|---|---|
|登录与目录|evidence/real-auth.json、real-browser/report.json|隔离Owner认证；现有目录与五页真实API读取|不代表所有写操作与全部页面状态通过|
|九类创作资源|evidence/real-catalog.json|各类型保存、同key读取、精确版本发布；草稿、版本、副本、归档的指定HTTP步骤通过|原报告总状态review_required；API证据不代替视觉、焦点和完整应用|
|真实React创作保存|evidence/real-ui-studio.json|正式React/controller/BrowserClient到Go/PG；结构化表单、JSON的false/0/null、发布与remount读回|这是JSDOM组件联真实HTTP，不是实际浏览器视觉|
|空白激活与运行世界|evidence/real-runtime.json、real-l04.json|指定空白激活、世界绑定和定义隔离步骤通过；实际会话NDJSON与持久消息指定步骤通过|不代表初始化七阶段全部真实完成，或所有模型场景完成|
|动态、通知与策略|evidence/real-l10.json|Owner发布、身份、策略、配置、通知等指定步骤通过|自主发布、NPC独立判断仍需来源证据；原报告review_required保留|
|上传与显示头像|evidence/real-media.json|真实PNG字节、授权读取、元数据与独立头像的专项步骤通过|L09后续视觉/生成分支未执行；报告blocked不能解释为这些步骤通过|
|实际浏览器上传|evidence/real-ui-upload/report.json|桌面上传与字节/原名/大小/SHA读回；窄屏丢回执后按key恢复，不重发字节|样本为隔离1×1 PNG，不代表真人素材或生成身份判断|
|PG契约|evidence/postgres-results.json|报告列出的六个实际PG测试pass|不把Go测试数作为全站业务完成率|

## 受控HTTP UI证据

各模块的`report-consolidated.json`清楚写明controlled HTTP。本轮额外检查聊天/通知/头像16场景、保留工具18场景、媒体生命周期6场景、档案与诊断边界8场景，均通过双视口执行。它们证明正式构建的展示、参数、回读、未知/失败与原请求恢复；fixture的queued→完成、模拟NDJSON、模拟删除和合成图片不证明真实模型、Worker或对象存储执行。

具体保留能力限制：

- 目标规划本次只确认自动规划开关保存、刷新、非法JSON保留与阻止提交；fixture未完整实现上限/恢复策略/CAS/候选入集，不能声称这些业务写入全部真实闭环。
- 视觉测试入口的受控服务没有成功的看图测试回执，本次确认原端点及失败反馈，不宣称真实视觉模型成功。
- 媒体引用、授权与删除均在隔离fixture里验证UI；真实MinIO删除与跨保护范围引用的最终权限不能据此判通过。
- 诊断导出/清理以受控快照与流验证完整性、二次确认和原key重试；没有清理真实业务诊断表。
- 真实媒体候选、规范参考、设定图与生成全链路仍没有本轮完整通过证据；按用户已纠正的UI主线，不为补这项而配置Comfy或调整后台规则。

## 最新发现的UI接入缺陷

F41目标次级记录未刷新、F42旧主快照错误释放、F43同页创建resume未重建控制器、F44取消失败提示被等待态隐藏、F47嵌套维护入口丢失，均已定位为前端状态/路由/适配缺陷。已修并有对应实际浏览器证据；没有证据表明这些问题由后端未写入或业务逻辑不通造成。

最新补充：F54–F59也仅修改发布/通知来源图标、角色菜单辨认信息和局部响应式规则。未知通知来源使用明确的自身属性判断，不能由对象原型产生组件。构建99已同步到18480，仍以real-browser/report.json记录五页只读实际API证据，不执行新的模型/媒体生成/激活。
