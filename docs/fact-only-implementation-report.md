# Fact-only 实施与本地验证报告

2026-10-13；分支 `codex/fact-only-reply-vision`。基线 origin/master 为 `e36b3be8b2efb91e02fbed2771352f49557a60df`；本地 master `6e11490c381fbb61cad73dda686b35846eda2f06` 保留。仅功能分支提交，没有合并/rebase master、覆盖稳定 latest 或部署真实环境。

## 已实现

- legacy 默认兼容；独立 `conversation.reply` / `vision.understanding` 设置与模型绑定，私聊 run 固定 schema/config/model/sampling；共享 generated queue 与独立 Writer 并发容量。
- Qwen 保留 Eino 原生工具与认知合同。fact-only reply 不接受正文、方向、语气或策略。Writer 从当前编译人格、活动 Goal、Actor/关系、权威状态及真实角色历史生成正文；required 预算不足明确失败，正常 e6 型号与 Unicode 表情保留。
- 普通文本经事务外生成/lease/持久缓存，事务内沿既有消息+receipt 发布，实际正文回填 Qwen。published replay 不调用模型。配置启用的 fallback 是认知模型执行 tool-free Writer，标记 actual binding/model/mode，不算独立模型验收；其实际 assignment 也固定到媒体续接。
- 最小媒体 envelope 产生 durable needs_action；opaque component refs 绑定真实 Capture/Kev/image intent；pending 不发布草稿。终态经独立 continuation 表和 `reply-continuation-v1` Temporal 队列恢复原生受限 reply 工具。原用户消息、source、Actor、turn、人格和配置身份保留，不新增用户轮次、不再次应用认知状态。正文+实际附件同一发布事务。
- 草稿绑定媒体清除旧 conversation/message/moment 自动发布目标；旧媒体 Worker 也只能完成生成。重复组件复用实际 media intent；原生媒体失败同时归一化 error 和 result 的 retryable，允许 Writer 消费真实失败。未安装 sticker 返回 unavailable；不伪造附件。
- 原轮次 `waiting_reply` 不被重claim；同会话新轮次、取消、到期和权限变化阻止晚发。原认知 final 失败明确终止未发布草稿，保留已提交消息。Temporal stable-ID start crash window、终态扫描、有限重试与失联恢复已接线。
- 独立视觉默认关闭。domain、原始 Provider assignment/secret 之前和实际排队提交三个入口治理 image 输入。只允许指定 vision binding；关闭/未配置零图像 HTTP。图像质量缺失走 skipped 后继续保存生成资产，视频配置不变。
- VisualIdentity generate/finalize 为文字阶段；review 才用独立视觉。缺失或不可用时保留真实 bytes-ready candidate + unverified/skipped provenance，停在 awaiting_review；不自动换 canonical，不反复新建 WakeUp/评审任务。direct review 需明确 manual；native review 需服务端 session/asset/config 来源。
- 设置 UI 显示模式、版本、独立模型及近期状态，提供 Owner/CSRF 保护的显式图片输入测试。诊断保留授权 Writer 输入、raw/parsed draft、真实 Tool result；成功响应的已知凭据回显、秘密键及图像载荷在存储前脱敏。UI提示模型候选与实际提交的区别。
- 0061/0062 为 additive schema。独立 Core/Web 不可变测试 tags、发布集合 manifest、fact-only-latest promotion head/digest 检查及实际镜像切换脚本已实现。

## 本地验证

定向 Go `-race` 用例通过，覆盖设置默认/严格校验、schema/registry 固定、上下文/required budget、真实 Generate/Stream wire、并发=1 嵌套 Writer、正文回填、envelope/e6/Unicode、截断/OOM/有限 transport retry、显式 fallback、组件 refs/真实等待/原 source、Temporal retry、独立视觉 physical gate、raw Provider secret 前拦截、成功响应脱敏及全 Browser OpenAPI 路由。

`pnpm test`：Browser Client 21/21，Core Client 2/2，Web 105/105。`pnpm typecheck`、`pnpm build`、Go `vet ./...` / `build ./...`、OpenAPI inventory、Compose bind sources 和 diff 检查通过。Web 构建有原有风格的 >500 kB chunk 提示，构建成功。

完整 Go 本地 `go test -race ./...` 已通过，显式设置 `FLUCTLIGHT_LIVE_PROVIDER_TEST=0` 和空 `GO_CORE_TEST_DATABASE_URL`；可选DB/真实模型/S3用例仍SKIP，不计入环境验收。旧诊断用例已更新为秘密字段脱敏，迁移head断言更新为0062并保留0061/0060升级链。

发布 helper 的 3 个 Python 用例本地通过。新模型与生产环境从未用于本轮代理验证。

PostgreSQL publication waiting/cancel 两个子用例、VisualIdentity candidate/provenance 用例因本机未提供 `GO_CORE_TEST_DATABASE_URL` 而 SKIP。MinIO/ComfyUI/Redis/Temporal 联机恢复、旧镜像实际消费、真实模型体验/稳定长聊、部署回滚由用户验证。SKIP不等于数据库闭环通过；色块 probe 不等于所有图片理解准确。

## 可核对的限制

确定结构与 authority/revision fence 可以检查；无法机械证明任意人物文字没有事实幻觉。真实拒绝、人格和 Goal 连续性、长聊自然度及视觉模型支持要在用户模型上验证。当前私聊只有 visible_output_committed，没有外部逐组件 delivery ack；正文/附件存储同事务，不声称网络送达。独立 release 两仓 alias 晋升不是原子操作，部分失败必须使用完整 manifest 中的 digest。

配置示例见 `fact-only-config/`，视觉调用点见 `fact-only-vision-call-sites.md`，用户检查步骤见 `fact-only-user-verification.md`，发布/回退见 `fact-only-deployment.md`。trace example 是明确标记的合成格式示例，manifest template 不是真实构建产物。

## 远端发布状态

用户已明确授权推送到公开 GitHub 仓库 `adougebabi/companion-chat` 并构建测试镜像。首次 push 源 SHA 为 `a53d9838501c144d24ae9d5314bf88f911379cf4`，运行 https://github.com/adougebabi/companion-chat/actions/runs/38401908262。Web、Go普通与race通过；Phase 8隔离PG门禁检出两条旧fixture合同问题：pseudo tool_calls错误原因已改为原生输出缺失；真实reply fixture含已禁用visible_text导致额外final repair。已保留无伪造副作用、精确两次调用/一条assistant断言，并增加inbox正常processed断言。修复后重建；此处不把尚未构建的镜像或模板记为真实发布证据。

流水线使用已有的一次性PostgreSQL补跑本机曾SKIP的三条新增受控合同用例（原source发布/取消、候选暂停/伪造评审拒绝、独立视觉实际wire）。真实Provider测试显式关闭；不连接用户真实环境。
