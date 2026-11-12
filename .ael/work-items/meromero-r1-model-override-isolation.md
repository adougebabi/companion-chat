---
{
  "intent": "change",
  "original_request": "用户只授权执行 R1。开始前必须保全 R0 并通过现有流程把父 T02 恢复为未完成，保留历史 pass 和受检版本；为 R1 单独登记状态。三阶段各正式保存回读配置、记录来源和版本，目标及同一对照任务真实串行执行并校验合同；两次独立新库、新任务、新请求。只允许 R1 测试局部夹具、状态与脱敏回执；出现环境阻塞必须记录原因及恢复条件后停止，不转向其他任务。",
  "target": "在一次性隔离数据库和本机真实 Provider 中，串行执行两轮独立 A/B/C 场景，证明 reflection variant 的 MeroMero→Qwen→MeroMero 覆盖与撤销不影响固定对照任务，并验证真实任务输出合同。",
  "id": "meromero-r1-model-override-isolation",
  "title": "R1 真实单任务模型覆盖、隔离及撤销恢复验证",
  "type": "test",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core/t02_live_model_override_integration_test.go",
    "R1 专用局部夹具与真实请求观察",
    "R1-model-override-review.md 与脱敏证据"
  ],
  "non_goals": [
    "不修改生产路由、默认模型、Prompt、采样、token 限额、回退策略、数据库结构、UI、Temporal、媒体、发布门禁或 Trace",
    "不覆盖或提交四个既有未暂存草稿，不连接生产业务库，不执行真实购买、换衣、消息发布或生图",
    "不得把 R1 通过当作 T02 或父项目完成；完成、失败或阻塞均停止"
  ],
  "acceptance": [
    "A/B/C 每阶段正式配置保存与回读，分别记录来源和版本",
    "每阶段 reflection 与同一无副作用正式对照任务真实请求使用预期模型并验证现有输出合同",
    "至少两次独立场景，各自新库、新任务、串行真实请求，记录 Provider 返回模型和无 SKIP",
    "保留失败并生成脱敏回执，父 T02 其他缺口保持未完成"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-t02-task-model-routing",
  "goal": "verified",
  "environment": "",
  "preset": "fast",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "r1-live-abc",
      "kind": "scenario",
      "command": null,
      "covers": [
        "A/B/C 每阶段正式配置保存与回读，分别记录来源和版本",
        "每阶段 reflection 与同一无副作用正式对照任务真实请求使用预期模型并验证现有输出合同",
        "至少两次独立场景，各自新库、新任务、串行真实请求，记录 Provider 返回模型和无 SKIP",
        "保留失败并生成脱敏回执，父 T02 其他缺口保持未完成"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry",
    "legacy-9b89c41b4be449b963dcba95"
  ],
  "feedback_of": "meromero-t02-task-model-routing",
  "definition_revision": 1,
  "max_retries": 2,
  "max_seconds": 0
}
---
# R1 真实单任务模型覆盖、隔离及撤销恢复验证
