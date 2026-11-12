---
{
  "original_request": "原 T03 要求按任务组织关系事实、保留真实业务内容和主体，不按字段后缀递归删除业务数据，Core 内部标识留在 Core。现有 Moment 关系证据走递归删键的默认路径，需核验实际请求。",
  "target": "",
  "id": "meromero-m2-15-moment-relationship-evidence",
  "title": "M2-15 动态关系证据的业务值与主体",
  "type": "feature",
  "source": "user",
  "scope": [
    "Moment Writer 已选关系证据的实体投影及最终请求",
    "隔离数据库、真实引用解析和受控 Provider 的定向回归"
  ],
  "non_goals": [
    "不修改默认模型、fact_only 开关、提示词文本、采样或 token 限额",
    "不重写所有证据类型或其他 Writer，不修改四处原有草稿",
    "不连接生产数据库或以受控 Provider 冒充真实模型"
  ],
  "acceptance": [
    "实际 Moment 准备路径解析当前作用域的关系引用，并将关系对象对应人物以语义身份投影",
    "保留关系业务对象中的原始嵌套 id、source_ref、revision、schema、false 等数据，不把它们当 Core 外壳删除",
    "排除关系外层数据库 ID、目标 actor ID、内部 revision 和来源外壳，保留已有引用作用域校验",
    "修复前红测、修复后隔离场景及直接相关回归实际执行并留证"
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
      "id": "m2-15-moment-relationship-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "实际 Moment 准备路径解析当前作用域的关系引用，并将关系对象对应人物以语义身份投影",
        "保留关系业务对象中的原始嵌套 id、source_ref、revision、schema、false 等数据，不把它们当 Core 外壳删除",
        "排除关系外层数据库 ID、目标 actor ID、内部 revision 和来源外壳，保留已有引用作用域校验",
        "修复前红测、修复后隔离场景及直接相关回归实际执行并留证"
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
# M2-15 动态关系证据的业务值与主体
