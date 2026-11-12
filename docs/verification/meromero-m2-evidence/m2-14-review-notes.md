# M2-14 已提交回复的持久流回归

同一隔离 PostgreSQL、受控 Provider 和 `TestDurableObserverReceivesDirectMainReply`
输入下，M2 起始提交 `838bb902b18280cbe5eeb119fb500db5da30ffa2` 与当前提交
`02db92f33193fca2baed4fc1b592312879c90047` 均通过、无 SKIP；命令和结果分别见
`m2-14-baseline.log`、`m2-14-current.log`。测试核对用户消息、已提交的完整
assistant 消息和 completed 帧，以及一次模型请求。两版相关持久观察器、测试、
主链入口和结算文件哈希相同，见两个 `m2-14-*-manifest.json`。

此前提交 `58ce5f98` 的浏览器 NDJSON 转换 11 个顶层测试和 Web 控制器/页面
60 项测试分别见 `m2-milestone-browser.log`、`m2-milestone-web.log`，均退出 0、
无 SKIP。`m2-14-browser-web-version-check.log` 记录上述受检浏览器与 Web 代码
到当前提交没有变化且工作区无对应改动；这些测试使用受控输入，不能代表真实
浏览器到真实 Provider 的端到端体验。

持久观察器只发送已提交的完整消息，不发送模型生成中的正文增量。这个缺口在
M2 起始代码已存在，关联 AEL `durable-chat-missing-live-token-stream`；本项不
修改它，也不将当前终态测试称为实时流式验收。M2 的完整流式体验仍未验收。
