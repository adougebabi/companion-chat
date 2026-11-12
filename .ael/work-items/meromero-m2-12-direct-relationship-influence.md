---
{
  "original_request": "原 T04 要求取消固定二段式后保留 influences、关系等认知结算及其证据来源和幂等；原 T03 要求按任务装配可回查的必要业务引用。当前代码显示 direct 请求可能过滤已有关系引用，尚需物理请求和持久化场景验证。",
  "target": "",
  "id": "meromero-m2-12-direct-relationship-influence",
  "title": "M2-12 direct 关系引用与认知影响结算",
  "type": "feature",
  "source": "user",
  "scope": [
    "direct 主对话当前有效关系的受控上下文引用、正式认知结算与幂等",
    "隔离 PostgreSQL 与受控原生 Provider 的物理请求及回归"
  ],
  "non_goals": [
    "不更改现有模型选择、采样、token 限额或提示词文本",
    "不改变 fact_only 开关及 takeover 等其他入口的行为",
    "不修改四处原有草稿、不连接生产数据库、不修复任务外非阻断问题",
    "不把受控 Provider 结果称为真实模型验收",
    "不改变 Kev 等现有上下文选择规则；未被选中的关系不虚称已经进入模型请求"
  ],
  "acceptance": [
    "当现有上下文选择规则保留当前授权关系时，direct 实际请求提供可绑定该关系且不泄露底层数据库标识的合法引用",
    "模型选择该引用作为 influence 后，正式事务记录对应结算和冻结证据，幂等重放不重复消息或结算",
    "非法、其他作用域或伪造关系引用不能绕过既有校验；无关系时普通聊天与显式 fact_only 模式保持可用",
    "隔离数据库执行修复前红测、修复后集成及直接相关回归，并保留版本和结果"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-t04-single-main-chat",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m2-12-relationship-influence",
      "kind": "scenario",
      "command": null,
      "covers": [
        "当现有上下文选择规则保留当前授权关系时，direct 实际请求提供可绑定该关系且不泄露底层数据库标识的合法引用",
        "模型选择该引用作为 influence 后，正式事务记录对应结算和冻结证据，幂等重放不重复消息或结算",
        "非法、其他作用域或伪造关系引用不能绕过既有校验；无关系时普通聊天与显式 fact_only 模式保持可用",
        "隔离数据库执行修复前红测、修复后集成及直接相关回归，并保留版本和结果"
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
  "definition_revision": 2,
  "max_retries": 3,
  "max_seconds": 0
}
---
# M2-12 direct 关系引用与认知影响结算
