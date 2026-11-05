---
{
  "original_request": "这个是什么东西，似乎一直在循环？两条相邻 visual_identity_agent 响应反复要求 visual_identity_generate_candidate，reason 均为 initial，后来停止。",
  "target": "核实同一视觉身份阶段重复工具调用的路径，防止已提交的候选生成等 checkpoint 继续被模型重复调用，同时保留基于真实状态的结果和合法纠错。",
  "id": "visual-identity-repeat-tool",
  "title": "防止视觉身份阶段完成后重复调用已提交工具",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core/visual_identity_agent.go",
    "apps/core-go/internal/core/visual_identity_tools_test.go",
    "apps/core-go/internal/core/visual_identity_test.go",
    "相关视觉身份规范和提示词组成清单"
  ],
  "non_goals": [
    "访问或修改用户业务数据库、运行中的任务或外部模型",
    "通过 token 上限、截断或预算拒绝阻止循环",
    "改变已确认的三图生成及审核业务规则",
    "自动提交、发布或提交图片"
  ],
  "acceptance": [
    "解释用户提供的重复模型响应及现场证据限制，受控重现已提交工具仍可被重复请求的路径",
    "checkpoint 真实推进后该阶段工具不再发送；即使模型无视空工具列表也不重复执行并保持真实持久状态",
    "checkpoint 尚未推进时保留必要工具及可纠正的错误反馈，不伪造通过或隐藏 provider 失败",
    "受控真实 HTTP 的流式与非流式测试及隔离数据库回归通过，保持已受理媒体任务的幂等恢复"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "id": "loop-physical",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core",
        "-run",
        "^(TestVisualIdentityPhysicalRequestStagesGenerateAndStream|TestVisualIdentityReviewPhysicalRequestRetainsImageAndModelEvidenceSchema|TestVisualIdentityCompletedCheckpointStopsRepeatedTool)$",
        "-count=1"
      ],
      "covers": [
        "checkpoint 真实推进后该阶段工具不再发送；即使模型无视空工具列表也不重复执行并保持真实持久状态",
        "checkpoint 尚未推进时保留必要工具及可纠正的错误反馈，不伪造通过或隐藏 provider 失败"
      ],
      "required": true,
      "timeout_seconds": 300,
      "artifacts": null
    },
    {
      "id": "loop-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "解释用户提供的重复模型响应及现场证据限制，受控重现已提交工具仍可被重复请求的路径",
        "受控真实 HTTP 的流式与非流式测试及隔离数据库回归通过，保持已受理媒体任务的幂等恢复"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "independent-generated-avatar",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 防止视觉身份阶段完成后重复调用已提交工具
