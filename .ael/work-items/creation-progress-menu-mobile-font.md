---
{
  "original_request": "图1，3个部分不在统一水平线上。更多准备操作应该用下拉菜单的方式处理",
  "target": "使窄屏中的更多准备操作触发器与同组操作使用一致字号。",
  "id": "creation-progress-menu-mobile-font",
  "title": "创建进度菜单窄屏字号对齐",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/web/src/styles/replica-creation.css",
    "创建进度操作窄屏页面验证"
  ],
  "non_goals": [
    "更改菜单业务行为",
    "提交图片、合并或发布"
  ],
  "acceptance": [
    "700px 及以下三个操作使用相同字号，390px 保持同一行且不横向溢出，320px 自然换行",
    "生产构建和浏览器场景验证通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "fast",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
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
        "生产构建和浏览器场景验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "mobile-font-ui",
      "kind": "scenario",
      "command": null,
      "covers": [
        "700px 及以下三个操作使用相同字号，390px 保持同一行且不横向溢出，320px 自然换行",
        "生产构建和浏览器场景验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "creation-progress-actions-menu",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 创建进度菜单窄屏字号对齐
