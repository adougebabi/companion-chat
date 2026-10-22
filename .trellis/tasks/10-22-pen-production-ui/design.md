# 实施设计

## 边界与拥有者
保留main.ts/App.vue/mountProduct正式入口，修改v3/proto components/pages、必要providers前端状态。共享视觉归styles/v3-proto.css，导航归workspace-navigation.mjs/app.js/shell.js；历史Vue views不作迁移目标。

## 数据与状态
现有service/DTO为事实权威，增加显示、草稿、来源、展开与比较状态。Owner/fid/persona/Actor/world/conversation/object隔离草稿，保留CAS/幂等/原请求身份。异步以当前页面对象epoch隔离，离开停观察；查询不触发写入。

## 收尾契约
视觉保留最近成功读取，按原identity/session/attempt观察，active/revision/artifact决定生效。助手比较基于实时完整name/body，共享焦点隔离弹层确认，取消不改草稿，采用只替换草稿。配置槽binding:用途、conversation.reply、vision.understanding独立；回执只清原提交草稿，保留期间新编辑，摘要用已确认DTO。

## 兼容/回退
保留原112功能/547动作追溯和有效路由，取消功能退出当前入口。开始时已有5项WIP保持原样。按组形成可审阅diff，回退限本轮。不部署或迁移数据库。

## 预览与证据
Vite直接加载正式源码，最终dist同源构建。每组先读Pen/将改源码，修改后立即CUA操作和视口检查。受控测试补边界，不能冒充真实业务。缺资源/权限则记录限制。

## 必要日程后端扩展（用户已批准）
跨日查询按角色实际时区和日期范围读取；非覆盖生成检查已有安排与在途请求，以原任务形成持久结果；约定安排检查接受状态、角色/人格/世界范围、关联安排与在途请求。复用现有领域函数、请求ID/幂等和版本边界，Browser boundary注册与生成客户端保持同步。前端按原请求只读观察，只有真实关联日程读回才呈现成功。新源码在隔离后端运行验证，不将NAS旧服务当作新契约验证环境。
