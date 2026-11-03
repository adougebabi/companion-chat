# 用户串行真实验收入口

真实 LLM/ComfyUI 最终验收尚未运行。用户要求资源优先、一次一个；以下命令由用户在资源可用时执行。不要把普通 Go 测试退出 0 或本文件存在理解为双 E2E 已通过。

## 当前会话已准备的环境

一次性测试栈为 `lac-agent-tool-5c2f9f296b`，与既有 `fluctlight-phase8-regression` 栈隔离。PostgreSQL 用例创建独立临时库；视觉 live 用例创建随机 S3 bucket 并清理。

Provider 和 ComfyUI 配置只读复用已有专用回归栈。私有配置在 `/tmp/lac-agent-tool-current.json` 指向的目录中，权限为 0600，不入仓库、不输出密钥。`visual-live.json` 保存原有 `media.comfyui` 工作流和本任务隔离 MinIO 配置；仅把容器用的 host.docker.internal 地址转换为本机执行的 127.0.0.1，未改工作流。非生成式 ComfyUI/S3 预检已通过（`visual-dependencies-preflight-host`），没有调用 LLM 或提交生图。

`/tmp/lac-run-final-live.py` 只装载这些私有环境变量并转发到仓库既有 `infra/acceptance/run-go-live-provider-smoke.sh`；没有新增另一套测试运行器。`/tmp` 文件仅供本机会话使用；若文件已清理，按 `visual-live-harness.md` 的配置说明重新准备环境并直接运行仓库脚本。

## 执行命令

分别运行，等前一个退出后再执行下一个：

```sh
# 单个 Tool
python3 /tmp/lac-run-final-live.py --suite tools --tool memory_event

# 单个正式认知 Agent：随机记忆、两轮查询、后续输入与答案验证
python3 /tmp/lac-run-final-live.py --suite agents --agent conversation_cognition

# 完整视觉身份 Agent：真实图片、多模态评审、canonical 与 character sheet
python3 /tmp/lac-run-final-live.py --suite agents --agent visual_identity

# 全部 18 个 Tool（schedule.replan 会请求真实模型）
python3 /tmp/lac-run-final-live.py --suite tools

# 全部 17 个 Agent
python3 /tmp/lac-run-final-live.py --suite agents

# 或一次执行两套完整验收
python3 /tmp/lac-run-final-live.py --suite all
```

上述入口使用 `-p 1 -parallel 1`，并通过跨进程锁阻止两个验收脚本同时抢用模型/GPU。缺少必需用例、零匹配、SKIP、BLOCKED、依赖错误或测试失败均返回非零。不要并行启动其他应用的模型请求；脚本锁不能控制仓库之外的 GPU 消费者。

每次运行产生新的证据目录，保留实际命令、退出码、受测代码摘要和 Go test 事件；失败尝试也保留，不覆盖成一次偶然成功。完整视觉行验证生产 handler 链，不单独证明 Temporal transport/history/worker recovery，相关 workflow 回归证据须分别看待。

## 资源清理

最终复验后可仅清理本任务一次性栈。使用 locator 中保存的精确 Compose 参数执行 `down -v`，先核对 project 等于 `lac-agent-tool-5c2f9f296b`。不要清理已有 `fluctlight-phase8-regression` 栈。清理后上面的私有数据库/MinIO地址需要重新建立才能复验。
