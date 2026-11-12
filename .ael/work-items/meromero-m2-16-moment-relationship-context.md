---
{
  "original_request": "原 T03 要求按任务装配关系事实，业务对象不能被按字段后缀递归删键，Core 内部标识留在 Core。M2-15 修复了已选关系证据；Moment 默认关系上下文仍经另一条递归删键路径。",
  "target": "",
  "id": "meromero-m2-16-moment-relationship-context",
  "title": "M2-16 动态默认关系上下文业务值",
  "type": "feature",
  "source": "user",
  "scope": [
    "Moment Writer 默认关系上下文的类型化投影与主体标识",
    "隔离数据库、正式 Moment 准备和受控 Writer 物理请求"
  ],
  "non_goals": [
    "不修改默认模型、fact_only 开关、提示词文本、采样或 token 限额",
    "不修改其他 Writer/全部上下文种类，不连接生产数据库",
    "不修改四处原有草稿或以受控 Provider 冒充真实模型"
  ],
  "acceptance": [
    "无需显式选择关系证据时，正式 Moment 准备请求保留默认关系中已有业务对象的嵌套键与 false 等值",
    "关系对象的目标人物按语义身份表示，未知人物不被猜测，Core 外层身份与来源外壳不进入该关系视图",
    "不改变关系选择范围、已选关系证据和正常动态发布；隔离场景保留红绿测试及直接回归"
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
      "id": "m2-16-moment-relationship-context",
      "kind": "scenario",
      "command": null,
      "covers": [
        "无需显式选择关系证据时，正式 Moment 准备请求保留默认关系中已有业务对象的嵌套键与 false 等值",
        "关系对象的目标人物按语义身份表示，未知人物不被猜测，Core 外层身份与来源外壳不进入该关系视图",
        "不改变关系选择范围、已选关系证据和正常动态发布；隔离场景保留红绿测试及直接回归"
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
# M2-16 动态默认关系上下文业务值
