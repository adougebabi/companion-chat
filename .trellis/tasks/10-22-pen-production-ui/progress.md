# 2026-10-22 实施进度（真实状态，持续更新）

## 授权
用户已批准最终计划并start。随后明确批准G2三个必要后端契约，见PRD追加。探子只读，主代理负责实现。

## 预览
Vite session 26417, http://127.0.0.1:5173/，代理NAS http://192.168.1.20:13001，用户已登录。CUA IAB browser id2, preview tab2（此前tab1随前轮结束消失，登录cookie仍有效）。当前视口390×844。真实角色fid fluctlight_99ae495ebe9b41c5188148c53cf6756c，仅只读/本地未提交编辑验收。

## 已实现/检查
G0正式shell稳定220px侧栏、菜单生活展开收起/共同经历，七组不变新菜单。G1双向自适应气泡、收到消息仅管理视角来源、本地聊天视角、scope隔离sessionStorage普通草稿、真实生活摘要/折叠/全天/只读小栈、窄屏完整摘要返回。输入框保持挂载。已读上报改为仅实际可见消息sequence，移除打开/流完成即整页全已读。
关于你默认只读七项，显式编辑，姓名/称呼分开、年龄数字、性别选项、多行补充；旧补充内容投影合并，在显式修改后才合并持久字段，其它隐藏情境保留。了解/关系只读抽屉不覆盖原草稿。
G2 outfit-composer已改内联候选（尚未浏览器验收）：普通即时选择收起/配饰多选保持/季节筛选/Escape收起/整身确认两槽/每场景草稿；仍需复核选中但不可用旧衣物可移除、对应测试，以及取消普通物品借还旧入口。

## G1真实CUA证据
1440×900实际聊天正常；管理切换保留未发草稿，来源1个，wrong按钮0；回交流隐藏来源。全天展开→折叠→窄屏展开仍保留全天。390 DOM innerWidth/scrollWidth均390。窄屏摘要一次paint重建丢返回按钮已修为固定共享控件。
关于你390真实读取成功，7项均真实未填写。进入编辑填写本地称呼，打开了解记录得到真实空态，关闭后草稿保留且焦点回触发按钮；取消编辑恢复未填写。未提交真实资料。该页innerWidth390、scrollWidth375（有滚动条，无溢出）。
期间NAS会话读取出现一次失败，现聊天catch只有提示无重试，待修恢复按钮。真实资源当前无日程/穿着/了解，不能算有记录分支通过。

## 自动检查
Web typecheck通过2次，首次共享+聊天修改build/noMock通过。相关30测试（nas-additions/v3-ux-scope/conversation-delivery）全通过。新about-you后typecheck session27223已退出0。尚无新增测试/最终全检。

## 下一步
先修G1 readback失败不能报成功、聊天读取重试；补核心行为回归测试和G1桌面检查。G2继续主体/日程/目标/衣柜，并加3后端契约。G3—7尚未改。

## 后端探索结论
当前日程GET/手动生成/约定排入均无专用契约。可复用readLifeContextTimezoneWith/readScheduleAtWith、generateInitialSchedule（expected_revision0保护）、acceptScheduleWithTx/replanScheduleWithTx、SurfaceDBTx与平台durable workflow intents。通用约定不能套intention.schedule，其action_plan闭集只有haircut/hair_dye/virtual_shopping。选择独立promise安排关联保留原执行契约，不能伪造action_plan。新增Owner schedule request durable表/intent/worker、权限版本幂等和状态查询，真实关联后才成功。
当前迁移Head0096_wardrobe_world_structure，下一0097；docker ps为空（无本地运行栈），需要隔离PG/Temporal。只列现有image不输出任何凭据。

## 四项收尾代码定位
visual现real-visual asyncPage评审reload会清旧图；需要局部保留成功快照+独立avatar+捕获session observer。反思list缺自动任务状态跟进。studio-jobs已有自动观察/采用确认与实时readDraft，但input不触发比较重画、比较是完整JSON，需变化摘要/未变折叠/取消恢复；overlay本身已inert/Escape/focus。config保存槽与新编辑保护已存在，主要补真实已保存摘要、分层说明、三槽反馈与autonomy模式同名。

## 后端实现中（尚未最终验证）
已新增0097_owner_schedules迁移（requests durable表，life_schedule_items.promise_request_id独立关联）、owner_schedules.go领域查询/请求/处理；生成复用initialSchedulePayload，未来day不覆盖；作用范围snapshot、life/CAS、pendingday、真实promise执行对象revision守卫；Worker schedule.owner与前/后边界注册；新增共享schedule-owner-contract并pnpm generate同步Browser/Core types、Browser methods（Core methods模板后加需再次generate）。
已编译四个Go包通过。隔离PG容器codex-pen-schedules-pg，127.0.0.1:15432，临时账号pen_test/pen_test_only，库pen_test，无卷，镜像pgvector/pgvector:pg16；非用户凭据。
新实际PG测试首轮发现typed nil map导致直接领域缺失值非nil（JSON仍null），已规范为真正nil；scope/CAS/boundary测试运行session7130待取。还须补真实accepted-promise安排测试、BFF真实链、迁移升级/幂等、Worker测试、客户端和frontend联调。
重要待查：新promise安排在原约定取消/改期后怎样保留/失效；必须避免自动变执行/履约。现validateSchedulePromiseLink只允许旧关联原区间或trustedworker引入新关联，preserveSchedulePromises禁止丢失未来关联/当前时段拆分，schema限定模型只能带已有promise引用。不要扩大成通用action_plan。

## 16:20 前后新增进度
隔离完整运行已启动，使用正式cmd/api+cmd/worker+同一Vite源码，不是Demo：localhost:5174（与NAS127.0.0.1:5173不同host隔离Cookie），Core127.0.0.1:18081，Vite session40765，API18100，Worker84699；私有测试环境文件/private/tmp/codex-pen-runtime.env（不要输出内容/提交，只有任务自产临时密钥）。依赖PG15432/Redis16379/MinIO19000/Temporal17233；容器codex-pen-schedules-pg、codex-pen-redis、codex-pen-minio、codex-pen-temporal，网络codex-pen-runtime。无用户数据/无生产卷。
临时种子脚本已移出正式Core目录至任务fixtures/pen_runtime_seed_test.go，仅明确测试人物pen_runtime_lan（隔离验收 · 林岚），正式auth Setup创建任务测试账户，测试密码pen-test-only-password。本地CUA登录成功，Tab3/localPreview绑定，IAB2，桌面1440×900。Tab2/preview仍NAS。Cua Playwright在Tab3的部分locator错误no_matches，原生AX索引click/setValue工作；请优先原生AX并每步刷新。
G2真实CUA：生活展开不跳页；日程4项真实持久化记录可独立展开（已过时段不会变完成）；日期输入2026-10-23先保留草稿，点击查找才换范围并真实无日程。实际提交一次生成请求schedule_request_0456238c587bfc249da5b64444e34a87，正式Worker经真实Temporal处理，因本地未配置Provider失败；UI自动由等待变失败，未生成日程/假成功，截图evidence/g2-schedule-real-worker-failure.jpg。这证明真实请求/Worker失败链，不是live模型成功。正向落库/约定安排由真实PG+受控模型测试验证，不冒充真实模型。
独立中途review发现P1跨日期改期旧关联挡新安排、P2跨午夜重放先被日期拒绝，均已修，新增真PG回归通过（TestOwnerSchedule共10s）。另原约定取消/窗口改变后新关联不作当前life authority。Promise投影新增scheduled_items，前端可查看真实关联/为已接受约定安排。
G2前端已接：LifeNavigation共用模块导航（在业务标题下），顶栏20px白底/业务标题28px，schedule日期/时间线独立展开/7天读取/意向约定生活倾向分区/生成保护/原请求观察/表单；goals纯卡片，意向与约定移日程；GoalOrdering组件选择/上下移/前置条件/循环拒绝，不再要求手输ID/JSON（高级恢复策略仍折叠）；wardrobe内联候选/季节/配饰/整身/每场景草稿，取消普通物品与借还入口。衣柜还需真实CUA检查，当前Tab3已进入衣柜正加载。
新typecheck session24875待取；G2相关测试日志/private/tmp/pen-g2-tests.log退出0；全BFF测试/private/tmp/pen-browser-tests.log退出0；较新正式build/noMock日志/private/tmp/pen-web-build.log退出0，但后续LifeNavigation/GoalOrdering等改动后仍须最终全检。
G3—7尚未实施！UX-R2视觉/创作比较/配置闭环主要尚未落代码。不得报告全站完成。仍需HTTP真实链集成/升级测试与更全面新日程反向矩阵、前端request草稿未知处理完善。Worker日志许多测试fixture无Provider触发的失败，不要全量反复输出。

## 最新实施交接（覆盖前文“G3—7未实施”的阶段记录）

用户将统一验收接回后，已继续完成G3—G7正式组件及四项收尾，详见delivery.md。主要新增：地点阅读/人物卡；共用月历纸张；真实故事快照统计/有效Owner记忆过滤；反思原任务观察；媒体卡与朋友圈单列；视觉参考/候选/独立头像分源自动读取并保留旧图；创建四步与独立真实进度；实时完整草稿比较/单一聚焦采用；配置三槽分层及保存摘要/角色共用自治标签；来源返回与游标/阅读快照。目标规划表单、排序与前置条件保稿，聊天生活摘要新增NPC只读小栈和返回阅读位置。

故事只读契约与aboutOwner查询、like_count已同步Browser/Core OpenAPI与生成类型；不从前端当前页估计总量。新增行为约束已记录.trellis/spec/frontend/pen-interaction-contract.md和backend/owner-schedule-story-contract.md。没有修改或丢弃开始时的旧任务WIP及profile-types.ts。

最后必要检查：62个修改/新增JS/MJS语法检查0错误；Web vue-tsc（亦项目lint命令）通过；正式build与no-Mock guard通过；Core/httpapi/browser/workflow的go test -run '^$'编译检查通过（没有执行业务测试）；git diff --check通过。按用户要求未继续CUA和全业务验收，也未运行完整最终测试套件。

同源码预览：NAS前端127.0.0.1:5173维持；本轮本地完整环境localhost:5174维持。API/Worker已从本次源码重启，exec session分别23438/2019；API监听127.0.0.1:18081。首次沙箱内启动无法连本地PG，已在已授权的沙箱外重启成功，没有审批拒绝。私有环境文件不输出不提交；日志/private/tmp/pen-api-current.log与pen-worker-current.log。隔离Provider仍未配置，不能宣称真实模型生成成功。

任务保持in_progress等待用户统一验收，不归档、不提交、不推送、不部署NAS。实施范围、可用预览、后端版本和验收边界见delivery.md。
