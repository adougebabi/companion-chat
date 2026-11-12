---
{
  "original_request": "原 T03 要求同一事实不在多个位置重复注入，同来源不同事实必须保留。Moment 当前既有默认关系上下文，也可接收同一关系的已选证据，需要沿正式请求核验。",
  "target": "",
  "id": "meromero-m2-17-moment-relationship-dedup",
  "title": "M2-17 动态关系上下文与证据去重",
  "type": "feature",
  "source": "user",
  "scope": [
    "Moment 默认关系上下文与已选关系证据的精确去重",
    "真实 Core 引用作用域、不同关系及物理 Writer 请求的隔离测试"
  ],
  "non_goals": [
    "不按相似文字、同一来源或相同业务值合并不同关系",
    "不改变模型配置、fact_only 开关、提示词文本、采样或 token 限额",
    "不扩展至其他上下文类型，不修改四处原有草稿或生产数据"
  ],
  "acceptance": [
    "同一 Core 关系快照同时存在默认上下文和已选证据时，物理 Moment Writer 请求只保留一份完整关系业务事实与可理解主体",
    "不同关系即使业务内容相同也均保留；重复或无效引用遵循现有校验，不伪造关系事实",
    "Core 关系标识和去重所需内部引用不进入 Writer 请求，输入源不被修改",
    "隔离 PostgreSQL 和受控 Provider 的红绿测试及直接相关回归实际执行并留证"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-t03-prompt-context",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m2-17-moment-relationship-dedup",
      "kind": "scenario",
      "command": null,
      "covers": [
        "同一 Core 关系快照同时存在默认上下文和已选证据时，物理 Moment Writer 请求只保留一份完整关系业务事实与可理解主体",
        "不同关系即使业务内容相同也均保留；重复或无效引用遵循现有校验，不伪造关系事实",
        "Core 关系标识和去重所需内部引用不进入 Writer 请求，输入源不被修改",
        "隔离 PostgreSQL 和受控 Provider 的红绿测试及直接相关回归实际执行并留证"
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
# M2-17 动态关系上下文与证据去重
