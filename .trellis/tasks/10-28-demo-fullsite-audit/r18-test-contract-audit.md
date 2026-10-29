# r18 旧交互测试核查

用户要求先核查，不在本次核查中修改生产代码或测试。基线 HEAD 7149a69e，判断对象为当前未提交 r18 实现。旧交互的判定依据是对应生产行为已经改为新流程；测试失败、历史编号、Sheet/Disclosure 关键字本身均不是删除依据。

## 发现时的六组过时断言（现已清理）

| 文件位置 | 场景 | 过时要求 | 当前实现与处理 |
| --- | --- | --- | --- |
| apps/web/test/demo-replica-inbox.test.mjs:19 | notification detail retains list and separates read acknowledgement from source navigation | 查看原始来源后必须 navigatePath 到独立 /diag/task 页 | InboxPage.jsx:55,63 已原位显示 TaskDetail；删除旧跳转断言，改测同一通知上下文、任务读取、返回、只标记一次已读且不处理任务 |
| apps/web/test/demo-replica-inbox.test.mjs:20 | notification and pending failed reads retain independent cursors and actual group retry | 两次精确查找“重新读取原列表” | ReadFeedback/ReaderFeedback 改为“重新读取通知来源”；按通知面板与对应 pending group 定位实际动作，保留原游标、独立来源、confirmed rows、server-as-of 断言 |
| verification/verify-chat-inbox-avatar-ui.mjs:49 | inbox-source | waitForURL /diag/task + 独立页返回来源页 | 改测 Inbox 内任务 Dialog 和返回通知详情；保留查看/取消不提交 workflow control 的验证 |
| verification/verify-chat-inbox-avatar-ui.mjs:55 | inbox-source-errors | 无来源链接时逐字要求旧长文案 | 删除旧文案断言；核验真实重新读取动作、非法链接不跳转、通知保留、标已读失败不清未读 |
| verification/verify-admin-deep-ui.mjs:10 | task-source | 独立任务页、旧“任务信息”页签和旧技术详情名称 | 当前为 /diag?view=tasks&taskId + TaskRecordDialog；改测背景列表、现行三页签、关闭仅清 taskId、保留上下文 |
| verification/verify-admin-deep-ui.mjs:18 | calls-and-health | “模型与媒体调用”旧页签、diag-tab 模式控件、旧服务回执文案 | 当前主页签“模型调用记录”，ChoiceSelect“调用类型”；服务页“工作区服务/工作区连接/完整连接检查记录”。重写入口与断言，保留实际调用和原就绪回执 |

以上 verification 路径均相对于当前任务目录。Inbox 单文件由只读核验代理定向运行：5 tests，3 pass，2 fail，失败对应前两项；未将失败自动判为废弃业务测试。

## 当前工作区已迁移，不能再列为待删除

- r11-controls: tbody/七个 td/摘要不能点击 → 实际 article、独立详情与恢复、无嵌套按钮。
- r12-regressions: 设置 Disclosure 折叠展开 → 设置分卡与类别切换保留输入、无写入。
- r11-flows: 生命周期事件默认充当执行阶段 → 实际执行记录页签，不伪造数字进度。
- demo-replica-management: 旧任务信息页签和技术详情文案 → 来源与调用及完整任务信息；无阶段时不造进度。
- r14-regressions: 恢复此环节 → 恢复原任务，仍验证实际阶段回调和 busy。
- r3-ux-recovery: 缺角色来源旧文案 → 未记录角色来源；完整原始响应与摘要隔离仍保留。

## 有效测试保留

独立 /diag/task/:id 仍支持直接访问；通用工作流在服务端 allowed_actions 明确允许时仍支持 restart/reset。不能将衣柜领域专属恢复的限制推广到全部工作流。DetailDrawer、OverlayHost 和 DomainPrimitives 仍有实际 Sheet 消费者，其 Escape、焦点、草稿与原节点测试有效。配置 CAS、accepted 回执 GET-only、原请求 body/key、权限和跨作用域保护均不属于旧 UI。

审计范围：当前修改的主要 UI 测试，以及未修改的通知、诊断、设置、创建相关测试与任务 verification 脚本；不是全仓全部测试无残留的证明。初次核查没有修改测试或生产代码。

## 用户确认后的清理结果

用户随后要求“这些旧的全删除，删除后告诉我，目前是否已经完成r18的处理”。上述六组旧断言及旧场景实现均已删除，原文件中保留有效用例，并用现行交互替换对应场景：

- Inbox 原独立任务跳转用例 → `notification task opens in the same dialog and returns without another acknowledgement or navigation`；保留任务实际读取、原列表、已读一次、返回正文和焦点。
- Inbox 原通用重试文案用例 → `each inbox reader retries its own failed source while preserving notification cursor and server read time`；明确分组与通知分页独立，原游标和服务端 as_of 不变。
- 浏览器旧 inbox-source / inbox-source-errors 场景 → inbox-task-context / inbox-invalid-source。
- 浏览器旧 task-source / calls-and-health 场景 → task-dialog-context / diagnostic-modes。
- 同时清除该管理脚本两处附带的过时逐字断言：任务受理反馈、无阶段记录提示，按当前真实状态验证。

未删除仍实际使用的独立任务直达路由、合法通用工作流恢复或 Sheet 基础组件测试。本次清理只修改测试、验证脚本与报告，没有修改生产逻辑。

验证：

- Web 全量 727/727，0 skipped；日志 `/private/tmp/fluctlight-demo-audit-20261028/r18-full-cleanup1.log`。
- 定向 Inbox 5/5；日志同目录 `r18-inbox-cleanup1.log`。
- 浏览器桌面1348×925与移动390×844，12/12；日志同目录 `r18-browser-test-cleanup1.log`。
- 浏览器详情报告：`evidence/admin-deep/report-r18-test-cleanup1.json`（8项）与 `evidence/chat-inbox-avatar/report-r18-test-cleanup1.json`（4项）。使用当前 production dist 与专有受控HTTP服务，服务只绑定本机并在结束后释放；不是真实Core/模型验收。
- 两个验证脚本 node --check 与 git diff --check 通过；独立只读复核未发现本次清理的阻塞项或业务保护丢失。

完成结论仅为本次旧测试契约清理完成；r18 仍有模块修复和全范围验收未完成，见 r18-remediation.md。
