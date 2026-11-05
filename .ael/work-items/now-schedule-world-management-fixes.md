---
{
  "original_request": "此刻时间轴与北京时间不符，删除生活时间线页面及入口；过去和当前日程状态不正确；世界地点没有角色，事件列表和详情样式不正确；管理中心标题应滚动缩放吸顶、左侧工具栏固定，仅右侧内容滚动，其他类似页面一并检查。",
  "target": "修复时间和日程的真实数据与状态呈现，移除生活时间线入口和页面，修复世界位置与事件体验，并按要求实现管理页面稳定滚动布局。",
  "id": "now-schedule-world-management-fixes",
  "title": "修复此刻日程、世界位置与事件以及管理页面滚动",
  "type": "bug",
  "source": "user",
  "scope": [
    "此刻和日程前后端读取与状态",
    "世界位置、角色及世界事件",
    "管理中心及类似工作台页面滚动布局",
    "相关自动化和真实页面验证"
  ],
  "non_goals": [
    "编造角色位置或事件执行状态",
    "修改无关模型提示词",
    "提交图片、合并或发布"
  ],
  "acceptance": [
    "此刻展示与角色时区及当前日期一致的真实日程，生活时间线页面及入口不可用",
    "过去、当前和未来日程的状态依据真实执行状态正确反映，相关失败与未知状态不伪装成成功",
    "世界地点展示真实角色位置，世界事件区域和详情符合用户提供的参考及现有真实操作",
    "管理中心及类似页面标题、侧栏与内容滚动符合需求并兼容窄屏",
    "受影响功能自动化检查、类型检查、构建及真实页面交互验证通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "",
  "risk": null,
  "ui": true,
  "ux": true,
  "checks": [
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
        "受影响功能自动化检查、类型检查、构建及真实页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "web-tests",
      "kind": "command",
      "command": [
        "pnpm",
        "--filter",
        "@fluctlight/web",
        "test"
      ],
      "covers": [
        "此刻展示与角色时区及当前日期一致的真实日程，生活时间线页面及入口不可用",
        "过去、当前和未来日程的状态依据真实执行状态正确反映，相关失败与未知状态不伪装成成功",
        "世界地点展示真实角色位置，世界事件区域和详情符合用户提供的参考及现有真实操作"
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
        "受影响功能自动化检查、类型检查、构建及真实页面交互验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "ui-scenarios",
      "kind": "scenario",
      "command": null,
      "covers": [
        "此刻展示与角色时区及当前日期一致的真实日程，生活时间线页面及入口不可用",
        "世界地点展示真实角色位置，世界事件区域和详情符合用户提供的参考及现有真实操作",
        "管理中心及类似页面标题、侧栏与内容滚动符合需求并兼容窄屏"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "ui",
      "kind": "ui",
      "command": null,
      "covers": [
        "世界地点展示真实角色位置，世界事件区域和详情符合用户提供的参考及现有真实操作",
        "管理中心及类似页面标题、侧栏与内容滚动符合需求并兼容窄屏"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "ux",
      "kind": "ux",
      "command": null,
      "covers": [
        "世界地点展示真实角色位置，世界事件区域和详情符合用户提供的参考及现有真实操作",
        "管理中心及类似页面标题、侧栏与内容滚动符合需求并兼容窄屏"
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
# 修复此刻日程、世界位置与事件以及管理页面滚动
