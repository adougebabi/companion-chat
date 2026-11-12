---
{
  "original_request": "M2/T05 原始验收要求关闭 Moment 不影响其他能力，插件停用不删除既有数据、不准入新工作，已提交结果可重放。",
  "target": "验证 Moment 停用后默认 direct 私聊及 Image 正式工具仍可用，既有动态保留且新动态拒绝。",
  "id": "meromero-m2-10-moment-disabled-image",
  "title": "M2-10 Moment 停用时默认私聊与 Image 保持可用",
  "type": "feature",
  "source": "user",
  "scope": [
    "正式插件设置保存与回读",
    "隔离数据库中 direct 私聊原生 Image 调用与回复",
    "Moment 既有资源、已提交回执重放及新工作拒绝"
  ],
  "non_goals": [
    "不修改 fact_only 开关、默认模型配置、Prompt 或采样",
    "不修改用户四处原有未暂存草稿",
    "不连接真实业务库，不将受控 Provider 冒充真实模型或真实生图",
    "不实施其他里程碑或不相关 bug"
  ],
  "acceptance": [
    "经正式设置关闭 Moment、保持 Image 启用，并回读实际配置",
    "默认 direct 私聊发现并调用 Image，消费实际 accepted 回执后正常回复；Moment 不作为新能力提供",
    "停用前已提交的动态及回执保留并可重放，停用后新动态被拒绝；图片新请求按正常准入执行",
    "隔离 PostgreSQL 与直接相关插件、主链回归通过；不宣称真实模型或 M2 整体验收通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-t05-image-moment-plugins",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "m2-10-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "经正式设置关闭 Moment、保持 Image 启用，并回读实际配置",
        "默认 direct 私聊发现并调用 Image，消费实际 accepted 回执后正常回复；Moment 不作为新能力提供",
        "停用前已提交的动态及回执保留并可重放，停用后新动态被拒绝；图片新请求按正常准入执行",
        "隔离 PostgreSQL 与直接相关插件、主链回归通过；不宣称真实模型或 M2 整体验收通过"
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
# M2-10 Moment 停用时默认私聊与 Image 保持可用
