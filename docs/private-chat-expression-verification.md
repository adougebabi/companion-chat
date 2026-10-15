# 私聊表达 v5：验证与表达版本回退

本次实现仍在 `codex/fact-only-reply-vision`。不自动合并、部署、调用真实用户发送或改写原人格/历史。v5测试镜像尚未构建；本地编译成功不能称为已经发布。

## 设置入口和在途行为
现有设置页“私聊回复模式”下增加“私聊表达版本”。选择 `fact-only-reply.v5` 使新Fact-only运行使用人物亲自发送消息的协议；选择v4仅回退表达合同，保持fact_only与当前模型/采样。旧配置缺字段的新运行缺省v5，旧durable draft缺reply_contract按v4兼容处理。

草稿首次创建固定协议hash/view/medium版本、配置revision和模型assignment。prepared/published直接读原结果，media恢复和失败lease重领不能重读新配置替换旧模型。当前事实、有效人格仍走原权限/版本检查和必要刷新。ModelRun诊断看prompt_version、reply_expression_contract、reply_persona_view、context_view；published消息及draft的committed_metadata才表示实际提交版本，候选的版本不代表已发送。

回退表达版本只保存现有 `conversation.reply` 对象的expression_version=v4，其余参数保留，不将mode改为legacy。进行中的v5 draft继续原合同，下一轮使用v4。回退到**旧二进制**则先沿 [既有回退步骤](fact-only-deployment.md) 停新任务、排空或取消在途任务并恢复旧兼容settings备份，旧decoder不认识新字段；不要修改已经发送的记录。

## 已执行的受控回放
```sh
mkdir -p "$PWD/.trellis/tasks/10-14-private-chat-persona-fidelity/verification"
FLUCTLIGHT_PRIVATE_REPLY_REPLAY_OUTPUT="$PWD/.trellis/tasks/10-14-private-chat-persona-fidelity/verification" \
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" \
go -C apps/core-go test ./internal/core -run '^TestPrivateChat' -count=1 -v
```

A/B/C/D各覆盖clean/dirty，每个组合3次，共24次。A原实际SDK请求已在业务修改前冻结；当前A与之逐字段相同。B只改协议，模型/采样/output cap/tool schema及其他messages不变。C加媒介/人格/状态视图并示范fixture原生detail查询；D加history来源语义并对已知旁白首稿示范一次修复。

所有响应都是明确的受控脚本，不计Cydonia首稿合格率、人格魅力或真实延迟。温知意材料来自任务书附录，属于重建fixture，不是原3705 tokens请求。三类其他人格为合成材料。

## 可选真实模型回放
使用既有reply live变量，保持同一冻结配置，输出只到本地证据目录，查询读重建fixture，发布为模拟。下列凭据由你的已有配置注入，不写入仓库：

```sh
FLUCTLIGHT_LIVE_PROVIDER_TEST=1 \
FLUCTLIGHT_LIVE_REPLY_PROVIDER_BASE_URL='YOUR_EXISTING_ENDPOINT/v1' \
FLUCTLIGHT_LIVE_REPLY_PROVIDER_MODEL='YOUR_EXISTING_CYDONIA_MODEL' \
FLUCTLIGHT_PRIVATE_REPLY_REPLAY_OUTPUT='/absolute/path/to/private-replay-output' \
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" \
go -C apps/core-go test ./internal/core \
  -run '^TestLivePrivateChat(ExpressionAblation|MultiRoundAndVoices)$' -count=1 -v
```

可选密钥使用 `FLUCTLIGHT_LIVE_REPLY_PROVIDER_API_KEY`。脚本不会自动调整采样，本fixture所有组使用0.8/0.9、1024输出预算、8192输入预算；这些是重建参数，不冒称原事故采样。A/B/C/D每组每类历史3次；温知意9轮和寡言、严谨、活泼三类合成人格分别3轮。只将当前输入与过去结果给模型，不把未来脚本塞入上下文。

首稿、失败输出、repair次数、actual requests、finish reason、模型返回usage（可用时）与耗时保留。请人工核对人物口吻、主动性/邀约/拒绝是否保真、现实事实和Goal方向；空criteria不计算客观完成率。本入口不真正生成或发送媒体，native媒体请求仍是private handoff；正式ready→恢复→提交链路继续使用原live HandleTurn/隔离环境入口。

## 隔离数据库回归
```sh
GO_CORE_TEST_DATABASE_URL='postgresql://TEST_USER:TEST_PASSWORD@127.0.0.1:5432/postgres?sslmode=disable' \
GOMODCACHE="$PWD/.gomodcache" GOCACHE="$PWD/.gocache" \
go -C apps/core-go test -race ./internal/core \
  -run '^Test(PrivateChatDurableStyleQuotaAcrossResume|PrivateChatCommittedStyleRepairAndMalformedSettlement|FactOnlyCommittedReplyMalformedFinalSettlement|ReplyMediaPublication|ReplyMediaCatalogDenial|FactOnlyProactiveSuppression)' -count=1 -v
```

必须是可丢弃测试数据库，不得指向生产。当前未配置此入口，相关用例SKIP；静态SQL/CAS核验和模拟不能替代PostgreSQL执行。真实聊天、部署和旧镜像回退演练仍按 [原验证分工](fact-only-user-verification.md) 由用户在隔离真实环境执行。
