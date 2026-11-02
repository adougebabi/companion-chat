# P0 回复提取回退验证（2026-11-02）

## 范围与根因

1. 重复 `# 回复`：原实现已将不同回复段合并为正文段落，并对完全相同的段落去重，符合要求。保留全部既有样例期望，新增原生发布、准备草稿及重复调用的数据库回归。
2. Writer JSON 畸形或额外字段：`parseReplyDraft` 原先因外壳格式失败直接报错，共享发布边界继而拒绝写入；前端 `replyBodyText` 对 JSON 解析失败直接返回空串。现在恢复顶层 `text` 字符串，经过相同的认知/事务记录/元数据过滤后正常发布或显示。支持缺失右花括号、未闭合字符串/围栏、字面换行及尾部杂项；不从嵌套字段提取文本，不恢复媒体行动权限。合法 JSON 的媒体字段仍经过原来的严格结构和业务校验。
3. 未闭合双语标题：中英文括号标签必须闭合才能被识别，可能留下标题和私有段。现兼容缺失右括号，保留正文并去掉协议段。单独的普通回复词 `reply` 原先被识别为空标题，现在作为正文保留；真正带标题标记的空回复、私有输出和无公开正文仍不发布。

## 文件与数据流

- `apps/core-go/internal/core/reply_body.go`：可恢复 JSON 文本读取、未闭合双语标签和单词正文边界。
- `apps/core-go/internal/core/reply_provider.go`：Writer 解码回退，忽略额外外壳字段，保留媒体验证。
- `apps/web/src/lib/reply-body.mjs`：历史/流入文本的对应防御过滤。
- `reply_body_test.go`、`reply_provider_test.go`、`reply_body_integration_test.go`、`testdata/reply_output_drift.json`、`apps/web/test/reply-body.test.mjs`：后端、前端共享语料、实际聊天 DOM、受控 Provider 传输和数据库回归。
- 同步 `.trellis/spec/backend/fact-only-reply-contract.md`、`.trellis/spec/frontend/feedback-ux-contract.md`、`docs/llm-prompt-composition.md`，替换旧的“外壳畸形即拒绝发布”规则。

已检查 `PublishConversationReplyTx` 及 Writer、原生 Tool、准备草稿调用路径；共享边界原本已调用 `parseReplyDraft`，无需修改调用方。数据库回归逐条断言消息正文等于清理后的精确文本、不同于原始输出，回执文本一致；原生调用重放不重复插入。5 类输入各走原生和准备草稿路径，共 10 条消息。原始候选仍只属于既有私有诊断路径，不进入消息正文。

## 检查结果

环境：`GOCACHE=/tmp/fluctlight-go-cache`；Node `/tmp/node-v24.12.0-linux-x64/bin/node`。

数据库由提供的 PostgreSQL 实例新建独立库 `p0_reply_verify_20261102_1427` 并迁移到 `0102_shared_worlds`，作为测试环境变量的目标。已有隔离测试 helper 另建和清理自己的临时库。运行完毕后删除本次新建库；前后数据库清单相同，保留原有 `lac_core_1257d3a81003528b1924` 及系统库，未清空原有数据库或无关行。

```sh
GO_CORE_TEST_DATABASE_URL='postgresql://box@127.0.0.1:55439/p0_reply_verify_20261102_1427?sslmode=disable' \
GOCACHE=/tmp/fluctlight-go-cache \
go test -race ./internal/core ./internal/httpapi/browser \
  -run 'Test(Reply|ConversationReplyPublication|Studio|Schedule|Scheduled|OwnerSchedule|AcceptSchedule|PostgresProviderQueue|ProviderExecutionTimeout|ProviderQueue|ProviderStreamExecutionTimeout)' \
  -count=1 -timeout=6m -json
```

- 上述 race：通过，Core 106 个顶层测试、55.098 秒；browser 9 个顶层测试、1.125 秒。含子测试总计 220 项通过，0 失败、0 跳过，无 race 报告。
- 包含近期 schedule、Studio 模板改名、模板保存校验、队列等待/执行超时及新回复回退测试。
- `go test -race ./internal/httpapi/browser -count=1 -timeout=3m`：完整包通过，1.773 秒。
- `go vet ./internal/core ./internal/httpapi/browser`：通过。
- `node --test test/reply-body.test.mjs`：68/68 通过，包含真实聊天 DOM 验证。
- 前端 `npm run typecheck`、`npm run build`：通过；生产 fixture 和 React 栈检查通过。使用 npm 是因为当前 pnpm/Corepack 会尝试写入只读全局目录。
- 前端完整 `npm test`：935 项，933 通过、2 失败、0 跳过，79.500 秒。
- `gofmt`、`git diff --check`：通过。

两项全量前端失败均在使用 `HEAD` 原版 `reply-body.mjs` 时原样复现（两份测试文件共 28 项：26 通过、同样 2 项失败）；随后逐字节恢复修改后的过滤函数，未改变这两项断言：

| 原有测试 | 失败断言与现状 |
| --- | --- |
| `test/r16-regressions.test.mjs:95` | `button(..., '使用日常生活世界')` 不存在，`assert.ok` 失败 |
| `test/r23-ui.test.mjs:48` | 期望 `/进行中/`，实际为 `状态待确认` |

新增共享语料 13 项；直接加载 `HEAD` 原版前端过滤函数验证，其中 11 项不能达到新期望，修复后的 13 项全部通过。原有 33 项共享语料的输入与期望均未变。

日志：`/tmp/p0-reply-go-race.jsonl`、`/tmp/p0-reply-browser-race.log`、`/tmp/p0-reply-vet.log`、`/tmp/p0-reply-web-targeted.log`、`/tmp/p0-reply-web-full.log`、`/tmp/p0-reply-unrelated-baseline.log`、`/tmp/p0-reply-typecheck.log`、`/tmp/p0-reply-build.log`。

## 既有断言变更及理由

以下 5 个输入原先要求报错，现在都要求成功且正文精确相等。旧期望与本次明确要求的“格式失败回退到清理正文”相冲突；没有删除输入或只检查非空。

| 测试 | 原输入 | 新精确期望 |
| --- | --- | --- |
| 原 `TestReplyDraftInvalidEnvelopeHasNoPublishableText`，现 `TestReplyDraftMalformedEnvelopeFallsBackToCleanBody` | `{"text":"partial reply"` | 成功，`partial reply` |
| 同上 | `{"text":"reply","direction":"private planning"}` | 成功，`reply`；不包含额外私有字段 |
| 同上 | 围栏中的 `{"text":"partial"` | 成功，`partial`；不包含围栏/JSON 外壳 |
| `TestReplyDraftMediaParsingNeverPublishesBrokenEnvelopes` | `{ "text": "断掉了"` | 成功，`断掉了` |
| 同上 | `{"text":"照片发了","direction":"先安慰"}` | 成功，`照片发了`；不包含额外私有字段 |

原媒体空 intent 拒绝断言、截断/Provider 失败断言、私有段不发布断言、原有正文精确匹配及幂等性断言均保留。另新增无正文、错误 text 类型和嵌套私有 text 的拒绝断言，以及精确数据库消息数量/正文/重放断言。

## UI 与操作约束

未新增 UI 或组件；聊天/历史继续复用既有 shadcn `Button`、`Input`、`Textarea`、`Badge` 及消息组件。没有替换页面结构。

未提交、暂存或改写 Git 索引；未修改 `/workspace/accept`、任何 `.env` 或图片。
