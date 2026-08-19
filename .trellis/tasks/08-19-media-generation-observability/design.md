# Technical Design: 媒体生成可观测性

## User Entry

复用 `COMPANION_DEBUG_INSPECTOR=1` 保护的本地开发检查器，不新增普通用户路由、终端日志或浏览器到 provider 的连接。检查器把“媒体意图与工作流摘要”替换为可读的媒体任务卡：卡片先显示 `kind · provider · status`、最终 provider 提示词、阶段、尝试次数、耗时、百分比或“provider 未报告百分比”、最新脱敏输出；原始 intent/workflow 摘要放入折叠的次级诊断区域。

打开检查器后，前端只轮询该人格的 debug-context 中媒体作业区域（约 1 秒一次），关闭 dialog 时清理 timer。更新卡片 DOM，不重建模拟事件或测试媒体表单，避免丢失表单状态。

## Durable Progress Snapshot

首版不新建日志表。每次 h3 lease attempt 在 `companion_jobs.result_json.progress` 保存有界对象：

```json
{
  "schemaVersion": 1,
  "attempt": 1,
  "stage": "generating",
  "stageLabel": "正在生成视频",
  "percent": null,
  "startedAt": "2026-08-19T10:00:00.000Z",
  "updatedAt": "2026-08-19T10:00:12.000Z",
  "elapsedMs": 12000,
  "latestOutput": "sampling frame 12 / 81",
  "latestStream": "stderr",
  "outputSeen": true,
  "outputLineCount": 8
}
```

`percent` 仅在 h3 stdout/stderr 明确输出百分号时更新并限制为 0–100；没有显式数字时保留 `null`，不得根据耗时推测百分比。完成阶段才可以确定为 100。

## Lease-safe Writer

新增内部 `recordMediaJobProgress(job, patch)`。它读取并合并当前 `result_json`，但只在 `id`、`status='leased'`、`lease_owner` 和未过期 `lease_expires_at` 全部匹配时写入 `result_json`、`updated_at`。它不续租、不改变 job status/attempt/run_after，也不写入 payload。每个 retry attempt 初始化新 snapshot，以免旧 attempt 百分比误投影到新 attempt。

`completePolledMediaJob()` 与失败 settle 路径必须合并已有 result/progress，而不是以新的 result JSON 覆盖它；最终阶段分别收束为 `complete`/`failed`，并保留最后安全输出和最终耗时。

## h3 Stream Capture

将 `runH3()` 改为同时 pipe stdout/stderr，并按换行与 `\r` 做行缓冲。每个非空片段会：

1. 去除 ANSI 转义和控制字符；
2. 使用现有敏感值脱敏逻辑，并额外掩盖绝对路径；
3. 取最新一条文本并限制为 480 字符；
4. 解析明确的 `12%`/`12.5%`；
5. 节流进度 SQLite 写入，普通输出最多约每秒一次，阶段转换、退出、超时和错误强制刷新。

h3 provider 在参数构造后写 `preparing`，子进程启动/输出写 `generating`，成功退出后写 `validating_output`，完成或失败后写最终阶段。完整命令、args、模型路径、输出路径和完整 stream history 永不持久化。

## Prompt Persistence

`submitMediaJob()` 在最终 prompt 编译后、外部 provider 调用前把 `finalPrompt`、精炼状态与初始 progress 合并到 source job 的 `result_json`。这使 h3 执行中即可显示真正送往 provider 的提示词，且同步 h3 成功写入不会丢失它。ComfyUI 保持现有 source-job + poll-job 行为；卡片可显示其已有 final prompt 以及等待/完成状态。

## Debug API Projection

`debugContextFor()` 继续最多返回当前人格的 10 个媒体 job。每项增加经脱敏、限长、兼容旧 job 的 `progress`：缺失时提供 `queued`/`unknown` fallback；`elapsedMs` 在读模型时由 `startedAt` 计算以反映当前运行时间，无需每秒仅为计时写数据库。latest output 上限 480，提示词和既有摘要继续走 `debugSummary`。

## Compatibility and Rollback

历史 job 没有 `result_json.progress` 时仍能正常显示/结算。ComfyUI 任务不要求 provider 进度输出。发生问题时，可停止进度写入而不影响现有 job claim、retry、provider submit/poll、附件落库和消息原位替换；检查器会退回旧摘要信息。
