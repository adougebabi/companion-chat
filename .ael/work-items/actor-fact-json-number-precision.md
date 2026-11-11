---
{
  "original_request": "用户要求任务外的非阻断问题只创建 bug，不在当前任务调整。T03 核对 Moment 证据时发现 readActorFactsWith 使用 decodeJSONValue（detail.go）将任意 value_json 通过 json.Unmarshal 解码为 interface；context_reference.go 的 addReferenceToRow 又用 cloneMap 对快照进行同类往返，factOnlyReplyEvidenceFact 使用 decodeObject。静态检查表明这些路径中的大整数可能先变为 float64 而失去精度。独立 Go 复现将 9007199254740993 解码再编码后输出 9007199254740992；尚未验证某条生产 Actor 数据受影响，也未修改通用数据层。",
  "target": "核实并修复 Actor 事实从持久化读取、引用快照到使用者的任意 JSON 数值精度，同时维持引用绑定和恢复的兼容边界。",
  "id": "actor-fact-json-number-precision",
  "title": "Bug：Actor 事实与上下文快照可能丢失大整数精度",
  "type": "bug",
  "source": "user",
  "scope": [
    "Actor value_json 的读取、上下文引用快照与相关消费者；评估其他使用相同解码辅助函数的受影响路径"
  ],
  "non_goals": [
    "T03 不修改通用 JSON 数据层或引用身份规则",
    "不修改真实业务数据",
    "不提交图片"
  ],
  "acceptance": [
    "在隔离测试中验证大整数、普通数字、false、null 和嵌套业务对象从存储读取到引用快照及消费者保持原值",
    "核对既有冻结引用、版本校验和恢复语义，明确处理兼容边界",
    "相关现有 Actor 事实和引用回归通过"
  ],
  "priority": 2,
  "dependencies": null,
  "parent": "meromero-t10-domain-regression",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "actor-fact-number-precision-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "在隔离测试中验证大整数、普通数字、false、null 和嵌套业务对象从存储读取到引用快照及消费者保持原值",
        "核对既有冻结引用、版本校验和恢复语义，明确处理兼容边界",
        "相关现有 Actor 事实和引用回归通过"
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
  "feedback_of": "",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# Bug：Actor 事实与上下文快照可能丢失大整数精度
