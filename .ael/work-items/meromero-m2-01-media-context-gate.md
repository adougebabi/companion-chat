---
{
  "original_request": "M2/T04 要求默认主对话迁移前，涉及完成声明或附件的内容通过对应核验后才能正式发布；M1 已覆盖当前输入含照片的场景，本项只处理实际发现的跨轮省略。",
  "target": "让上一轮媒体话题下省略图片名的已交付声明进入既有证据核验，避免无真实交付的声明正式发布。",
  "id": "meromero-m2-01-media-context-gate",
  "title": "M2-01 修复跨轮媒体指代的发布核验",
  "type": "bug",
  "source": "user",
  "scope": [
    "direct 媒体声明检查的相关会话上下文判定和正式路径测试",
    "隔离 PostgreSQL 和可用的本地 MeroMero 检查器验证"
  ],
  "non_goals": [
    "不扩展同 ID 重放等已暂缓的非阻断缺陷",
    "不修改模型 Prompt、采样、token 限额或全局默认模式",
    "不重构插件、不连接生产数据库、不覆盖四处原有草稿"
  ],
  "acceptance": [
    "上一轮讨论图片、本轮省略图片名称而声称已贴到聊天时，无真实交付不能作为正式回复发布",
    "已有匹配交付、否认或普通无关聊天不被一律拦截；原 direct 媒体声明回归保持通过",
    "隔离 PostgreSQL 的正式 HandleTurn 复现与修复后验证，按能力保留真实 MeroMero 检查证据"
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
      "id": "m2-01-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "上一轮讨论图片、本轮省略图片名称而声称已贴到聊天时，无真实交付不能作为正式回复发布",
        "已有匹配交付、否认或普通无关聊天不被一律拦截；原 direct 媒体声明回归保持通过",
        "隔离 PostgreSQL 的正式 HandleTurn 复现与修复后验证，按能力保留真实 MeroMero 检查证据"
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
  "feedback_of": "meromero-m1-01-direct-media-evidence",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# M2-01 修复跨轮媒体指代的发布核验
