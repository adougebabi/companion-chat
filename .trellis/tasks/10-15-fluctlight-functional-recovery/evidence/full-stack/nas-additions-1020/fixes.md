# NAS追加报告修复记录

源码修复对应UX-07—16，逐项结果见result.json。原报告及其ID保留，没有修改NAS数据、配置、凭据或重放请求。

## 真实根因

1. 当前目标列表误将history=true解释为包含历史，实际Core为终态专属分区。改为false，真实BrowserClient受控传输验证查询、分页游标及503传播；不回退detail.goals。
2. catalog names:null本应只列目录，却输出全schema。NAS当次工具结果38377、总输入51360、最大18432。删不请求的schema仍有8752估算token长说明，因此为49内置能力显式写用途概览；会话38项目录现4080估算token。名称和用途完整；按名原说明/schema保留，未改模型上限/工具事实/ADK/Qwen/Cydonia边界。
3. 日程工作流为长期运行，接受日程后等待本地午夜。失败账本此前不再被核对，RUNNING分支也不修failed。仅真实RUNNING且RunID非空时通过类型+状态CAS作观察修复，清completed_at/next_attempt_at/当前error，不计新attempt；历史失败事件保留。观察错误/terminal不重派或伪完成。
4. 缺头像空src、独立stack等待区、缺scheduled展示字典、物品无登记空态、全局a:hover分别由共享Avatar/聊天反馈/状态字典/Inventory/局部CSS修复。日程窗口标记使用原RFC3339瞬时值，不从墙上时间推断执行。

## 未冒充修好的部分

- 动态req_4a880d15c66f8a3369c9638495147662底层为真实Provider connect: connection refused（原服务地址未进入证据文件）。新增失败原因诊断入口，保持failed。服务恢复及再次建议由用户统一回归，不自动重放。
- 海边散步地点家没有发现代码兜底路径。保留已接受记录，未来规划提示要求地点与活动一致、未知可空且当前地点不可复制至全日；实际模型遵循待测。
- 日记当时未日终且pending/attempt0，不强行生成或宣称故障/完成。
- 当前代码没有部署NAS；浏览器截图为正式产物+隔离传输夹具。390px width/scroll=390，13计划/1当前标记/13操作折叠；不是104路由重新功能验收。

## 验证

Web253/253无skip，type/build/no-Mock；Core纯/受控10根、Workflow纯/受控7根，无真实DB/Temporal/模型。Core全包build通过。受控SQL命令不证明PostgreSQL并发/生产修复成功。两名只读复核，前端发现排队新消息可能导致诊断链接错turn，已依据原retry/active identity及匹配的真实失败记录修复，添加单独用例。

浏览器结果和限制见result.json。截图保存于同目录。诊断跳转核对了实际eventType+request-ID筛选；该诊断列表的完整数据不在当前夹具覆盖内。iframe回放期间CUA日志曾记录MutationObserver.observe参数非Node，来源未归因，不声称console-clean。没有额外模型调用。

## 用户部署后优先复测

Core/Web/业务Worker更新到同一提交。原回归角色依次检查目标列表与规划一致、日程长期任务/真实Workflow同为运行、当前/历史时段、头像与等待/失败状态；原失败记录继续保留。模型/图像、动态、日记按原集中回归流程继续，不能因本地检查通过视为真实功能通过。
