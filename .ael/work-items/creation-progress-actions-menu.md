---
{
  "original_request": "图1，3个部分不在统一水平线上。更多准备操作应该用下拉菜单的方式处理",
  "target": "统一角色创建进度区域三个操作的水平对齐，将更多准备操作改为下拉菜单并保留原行为。",
  "id": "creation-progress-actions-menu",
  "title": "创建进度操作对齐并改为下拉菜单",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/web/src/react-app/views/CharacterWizard.jsx",
    "apps/web/src/styles/replica-creation.css",
    "apps/web/test/demo-replica-creation-ui.test.mjs",
    "创建进度操作的桌面和窄屏交互验证"
  ],
  "non_goals": [
    "改变角色创建或准备进度的业务语义",
    "修改其他页面的共享菜单组件",
    "提交图片、合并或发布"
  ],
  "acceptance": [
    "同一行的查看资料、返回角色库和更多准备操作垂直居中对齐，窄屏无横向溢出",
    "更多准备操作以可键盘使用的下拉菜单呈现，重新读取进度、角色详情、开启另一段相识的原功能与条件保持",
    "打开和关闭确认弹窗后焦点能正确返回，创建资料与返回链接不受影响",
    "相关测试、类型检查、构建和真实页面交互验证通过"
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
      "id": "creation-ui-tests",
      "kind": "command",
      "command": [
        "node",
        "--test",
        "apps/web/test/demo-replica-creation-ui.test.mjs"
      ],
      "covers": [
        "更多准备操作以可键盘使用的下拉菜单呈现，重新读取进度、角色详情、开启另一段相识的原功能与条件保持",
        "打开和关闭确认弹窗后焦点能正确返回，创建资料与返回链接不受影响"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "web-typecheck",
      "kind": "command",
      "command": [
        "pnpm",
        "--filter",
        "@fluctlight/web",
        "typecheck"
      ],
      "covers": [
        "相关测试、类型检查、构建和真实页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
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
        "相关测试、类型检查、构建和真实页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "creation-actions-ui",
      "kind": "scenario",
      "command": null,
      "covers": [
        "同一行的查看资料、返回角色库和更多准备操作垂直居中对齐，窄屏无横向溢出",
        "更多准备操作以可键盘使用的下拉菜单呈现，重新读取进度、角色详情、开启另一段相识的原功能与条件保持",
        "打开和关闭确认弹窗后焦点能正确返回，创建资料与返回链接不受影响",
        "相关测试、类型检查、构建和真实页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    }
  ],
  "knowledge": [
    "lac-project-entry"
  ],
  "feedback_of": "",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 创建进度操作对齐并改为下拉菜单
