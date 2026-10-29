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

- 目标规划已补齐上限/恢复策略/手动active排序/依赖、候选与来源、CAS、原请求未知重放和accepted GET-only等受控UI回读，实际8/8通过。原夹具不足已修正；仍不证明真实Core/PG规划和模型完成。
- 视觉测试入口的受控服务没有成功的看图测试回执，本次确认原端点及失败反馈，不宣称真实视觉模型成功。
- 媒体引用、授权与删除均在隔离fixture里验证UI；真实MinIO删除与跨保护范围引用的最终权限不能据此判通过。
- 诊断导出/清理以受控快照与流验证完整性、二次确认和原key重试；没有清理真实业务诊断表。
- 真实媒体候选、规范参考、设定图与生成全链路仍没有本轮完整通过证据；按用户已纠正的UI主线，不为补这项而配置Comfy或调整后台规则。

## 最新发现的UI接入缺陷

F41目标次级记录未刷新、F42旧主快照错误释放、F43同页创建resume未重建控制器、F44取消失败提示被等待态隐藏、F47嵌套维护入口丢失，均已定位为前端状态/路由/适配缺陷。已修并有对应实际浏览器证据；没有证据表明这些问题由后端未写入或业务逻辑不通造成。

最新补充：F54–F59也仅修改发布/通知来源图标、角色菜单辨认信息和局部响应式规则。未知通知来源使用明确的自身属性判断，不能由对象原型产生组件。构建99已同步到18480，仍以real-browser/report.json记录五页只读实际API证据，不执行新的模型/媒体生成/激活。


## 2026-10-29 F60–F67与build104实接检查

World最小retry回执、Goal原请求恢复、Self详情/次级历史刷新均为前端消费和状态缺陷；没有修改Core、数据库或后台规则。Studio只提升验证夹具的目标运行态联动，未复现生产Studio缺陷。受控实际浏览器：World14、Goal8、Studio2、Self10通过；具体请求/回读断言和证据路径见coverage.state_verification。

最新build104仅静态同步既有18480 Web并恢复原runtime-config。evidence/real-browser/report.json为这次实际读取，五页通过，记录API均200，无浏览器异常；Owner/角色来自既有真实目录。Core/PG/Worker未重启，本次未发聊天、模型生成或激活。真实媒体模型/存储端到端证据仍按上面边界保留，未用受控通过替代。


pass116更新：F68首次目录读取失败与F69修正JSON后旧错误均为UI状态缺陷；没有改变认证协议、Core或提示词。新增受控应用异常、账户、目录、Profile、七资源类型、换装的50条浏览器结果通过，分别记录在coverage。换装no-change使用与当前穿着相同的目标，未用互相矛盾的fixture制造“已经穿着”。未知POST严格保留原body/key，accepted GET失败不新发请求。真实五页读取的最新报告来自build114，后续build116仅错误卡对齐。


## pass125收敛清单结果

新增44条受控生产浏览器检查通过，没有新增生产代码修复。管理配置按实际PUT端点，通过有界测试receipt/CAS状态验证等待、新输入保留、unknown原回执恢复、明确拒绝后停止未受理、显式重基线、endpoint/binding部分已确认不重复写；没有改真实部署配置。备份只对内存queued job执行取消，running只读，未运行维护工具。定义包只进受控fixture，保持原pack/digest/reason/key恢复且仅独立草稿，未发布/应用。动态使用正式processing/rejected状态，不注入不存在的running/declined owner-request枚举；不把这些模拟结果当模型判断。真实18480最新build116五页只读通过。
