---
{
  "original_request": "现在媒体任务失败了，完全不能恢复（看图1）。应该要像图2、3、4那样。处理这个问题。",
  "target": "让失败的真实媒体任务能从任务详情安全恢复，并清楚显示真实执行阶段、记录和可用操作。",
  "id": "media-task-failure-recovery",
  "title": "修复媒体任务失败后的恢复与任务详情操作",
  "type": "bug",
  "source": "user",
  "scope": [
    "媒体任务后台执行与恢复契约",
    "任务中心媒体任务详情、状态和操作",
    "相关自动化与真实页面回归"
  ],
  "non_goals": [
    "提交图片",
    "发布或合并",
    "伪造阶段、运行记录或恢复结果"
  ],
  "acceptance": [
    "失败的媒体任务从任务详情可执行后台实际支持的恢复操作，已完成工作、既有数据和记录按恢复语义保留",
    "任务详情的执行阶段、执行记录、任务信息和恢复操作如实呈现媒体任务的状态、失败原因与可用能力",
    "对可恢复和不可安全恢复的情形有针对性回归，相关检查通过并验证实际页面交互"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": true,
  "ux": false,
  "checks": [
    {
      "id": "recovery-tests",
      "kind": "scenario",
      "command": null,
      "covers": [
        "失败的媒体任务从任务详情可执行后台实际支持的恢复操作，已完成工作、既有数据和记录按恢复语义保留",
        "对可恢复和不可安全恢复的情形有针对性回归，相关检查通过并验证实际页面交互"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "web-typecheck",
      "kind": "command",
      "command": [
        "pnpm",
        "--filter",
        "@fluctlight/web",
        "typecheck"
      ],
      "covers": [
        "任务详情的执行阶段、执行记录、任务信息和恢复操作如实呈现媒体任务的状态、失败原因与可用能力"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "web-build",
      "kind": "command",
      "command": [
        "pnpm",
        "--filter",
        "@fluctlight/web",
        "build"
      ],
      "covers": [
        "任务详情的执行阶段、执行记录、任务信息和恢复操作如实呈现媒体任务的状态、失败原因与可用能力"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "recovery-ui",
      "kind": "ui",
      "command": null,
      "covers": [
        "任务详情的执行阶段、执行记录、任务信息和恢复操作如实呈现媒体任务的状态、失败原因与可用能力",
        "对可恢复和不可安全恢复的情形有针对性回归，相关检查通过并验证实际页面交互"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 修复媒体任务失败后的恢复与任务详情操作
