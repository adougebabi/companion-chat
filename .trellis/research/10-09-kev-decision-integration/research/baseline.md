# 规划期基线（2026-10-09）

基线 commit：63be0df，branch master。开始前产品工作区 clean；本轮仅新增/更新任务规划材料。

| 检查 | 结果 | exit |
| --- | --- | --- |
| go -C apps/core-go test ./internal/ai/agent ./internal/workflow -count=1（沙箱外重跑） | 两 package PASS：agent 0.124s，workflow 0.484s | 0 |
| pnpm typecheck | browser-client tsc、core-client tsc、Web vue-tsc 全部 Done | 0 |
| /tmp/kev-eino-compat 内 go test -v -count=1 ./...（沙箱外） | TestNativeLoopCallTimeToolsGrow、TestNativeBatchGateBlocksAllTools，2 PASS | 0 |

试验 raw 结果：

```
=== RUN   TestNativeLoopCallTimeToolsGrow
native loop wire schema=[[discover] [discover hidden] [discover hidden]]; real tool executions=2
--- PASS: TestNativeLoopCallTimeToolsGrow (0.00s)
=== RUN   TestNativeBatchGateBlocksAllTools
--- PASS: TestNativeBatchGateBlocksAllTools (0.00s)
PASS
ok github.com/fluctlight/local-ai-companion/apps/core-go 0.164s
```

试验代码快照：eino-v0737-compatibility-test.go.txt。可复现：建临时目录，复制 apps/core-go/go.mod 和 go.sum，再将快照复制为 compat_test.go，执行 go test -v -count=1 ./...。使用本地假 HTTP 服务和真实项目锁定 OpenAI component；不调用 Kev 或真实 LLM。

首次沙箱内本地监听 operation not permitted；首次 workflow cache 读取 operation not permitted。后续沙箱外重跑均通过。没有将权限失败归为代码失败。

未执行：全 Go suite/race/vet/build、pnpm test/build、真实 Postgres/Redis/Temporal/Compose、真实 Kev、生产 persona switch/stream/candidate gate、本地全启用 settings 应用。以上留在实施验收，不以此次基线代替。

Task context validate 首次通过但提示 autonomy spec 超出 32768-byte 注入限制；现已保存逐行完整分片并重新验证，避免丢失尾部规范。
