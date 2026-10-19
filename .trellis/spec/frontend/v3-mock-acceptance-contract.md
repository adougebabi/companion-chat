# v3 Mock 页面验收边界（历史阶段）

> 已被用户 2026-10-15 的完整全栈交付请求替代：现在以补全后的当前 v3 加 UI-01/UI-02 为 UI 基线，允许且要求接回真实后端；不再等待逐页 Mock 验收。此文保留旧阶段记录，禁止用其 Mock-only 条款阻止本轮实施。当前执行依据见 `.trellis/tasks/10-15-fluctlight-functional-recovery/full-stack-delivery-source.md`。

用户明确要求先 1:1 恢复已确认 v3、组件化并补齐遗漏页面，数据全部 Mock；页面由用户验收通过后才开始真实接口填充。本规则优先于旧“已接入”标记与本阶段不适用的实时 API 指南。

- 活跃入口为 `main.ts → App.vue → v3/proto/app.js`。不把旧 Vue 业务视图替换进 v3，不因为旧 Core 有某表单而改变已确认页面。
- 为保留原型源码与 DOM，展示组件沿用原生 ES Module 函数，位于 `v3/proto/components/`，页面组合这些组件。组件 slots 只接受受控组件生成的 HTML；所有用户文本/属性值必须 `esc()`。不可把整页套壳称为完成组件化。
- 公共 Field、Tabs、Button、Card、ImageCard、NPCRow、MessageLog、Diff、Revision/Conflict、来源和浮层必须实际复用。按钮默认 type=button；辅助按钮不可默认提交表单；number 允许小数，再由业务校验范围。
- Mock 服务位于 `v3/proto/mock/`；唯一 saved store 位于 `v3/proto/store.js`。页面 Map 只保留未提交草稿，并在演示重置时清理。禁止另建平行的已保存业务状态。
- Mock localStorage 键为 `fluctlight.v3.acceptance.mock.v1`，不能读写真实数据键。持久化失败应报错，内存回滚，不能假成功。
- 关于你按角色+Human Actor 隔离；版本冲突保留草稿；重读只推进版本，不自动保存。
- NPC 定义、发布、独立运行人物、关系分别处理。创建/应用不结识；有来源的接触演示必须明确操作。浏览世界不移动角色。退休角色只读；引用对象优先归档，不能物理移除。
- 路由切换通过 `closeOverlays()` 关闭浮层，保证栈、确认 Promise 和焦点正常清理。
- 保留 C 鼠尾草青绿、原 CSS 和资产；新增页延续相同结构，不设计另一套视觉系统。
- 验收台账为 `docs/v3-page-acceptance.md`。区分原型 UI 测试、Mock 服务测试、保留的旧组件测试和真实业务联调；本阶段不得宣称模型/API/数据库联调通过。
- Core/BrowserClient/旧 Vue WIP 保留冻结；用户明确页面验收通过前不得重新接回真实 transport。

## 当前真实接入的视觉与验证补充（2026-10-19）

- “关于你”的真实数据适配和冻结 Mock 共用 `AboutYouCard`。默认保留五项资料、头像卡片与“更多基本资料”，完整字段分配到三个标签和折叠区；不能因接入 19 个真实字段改成长表单。保存说明、实际冲突重读与来源历史仍可操作。
- 配置编辑器必须按实际资源显示 `unconfigured`、`runtime_default`、`stored`；新端点、未绑定角色和未配置凭据不能使用默认 `stored` 标记。保存及重读应更新当前来源，不能覆盖用户期间编辑的草稿。
- 浏览器真实验证、受控组件/HTTP/PG 测试与用户执行的模型验收分别记录。脚本预检须接受匿名 `/auth/session` 的 401，并先导入用户指定 Cookie；503/连接错误仍是失败，不能转为未登录。
