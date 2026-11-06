---
{
  "target": "管理中心各功能页面在纵向滚动内容时，页面标题、说明、操作入口与功能标签保持固定可见，内容在其下方滚动。",
  "id": "admin-center-sticky-section-headers",
  "title": "固定管理中心各功能页的顶部区域",
  "type": "feature",
  "source": "user",
  "scope": [
    "管理中心功能页面的共享布局与标题/操作/标签区域",
    "相关响应式布局与管理中心页面交互验证"
  ],
  "non_goals": [
    "更改左侧管理中心导航的固定方式",
    "更改管理中心以外页面的布局",
    "提交图片、合并或发布"
  ],
  "acceptance": [
    "诊断与任务页的标题、说明、右侧操作入口及标签栏在主体纵向滚动时始终位于内容上方并保持可见",
    "管理中心其他功能页采用一致行为，切换功能、标签和操作仍正常",
    "窄屏与桌面布局下固定区域不遮挡内容、不产生错误滚动容器或层叠问题"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "fast",
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
        "诊断与任务页的标题、说明、右侧操作入口及标签栏在主体纵向滚动时始终位于内容上方并保持可见",
        "管理中心其他功能页采用一致行为，切换功能、标签和操作仍正常",
        "窄屏与桌面布局下固定区域不遮挡内容、不产生错误滚动容器或层叠问题"
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
        "诊断与任务页的标题、说明、右侧操作入口及标签栏在主体纵向滚动时始终位于内容上方并保持可见"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "admin-center-sticky-ui",
      "kind": "scenario",
      "command": null,
      "covers": [
        "诊断与任务页的标题、说明、右侧操作入口及标签栏在主体纵向滚动时始终位于内容上方并保持可见",
        "管理中心其他功能页采用一致行为，切换功能、标签和操作仍正常",
        "窄屏与桌面布局下固定区域不遮挡内容、不产生错误滚动容器或层叠问题"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "human",
      "id": "ui",
      "kind": "ui",
      "command": null,
      "covers": [
        "诊断与任务页的标题、说明、右侧操作入口及标签栏在主体纵向滚动时始终位于内容上方并保持可见",
        "管理中心其他功能页采用一致行为，切换功能、标签和操作仍正常",
        "窄屏与桌面布局下固定区域不遮挡内容、不产生错误滚动容器或层叠问题"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "human",
      "id": "ux",
      "kind": "ux",
      "command": null,
      "covers": [
        "诊断与任务页的标题、说明、右侧操作入口及标签栏在主体纵向滚动时始终位于内容上方并保持可见",
        "管理中心其他功能页采用一致行为，切换功能、标签和操作仍正常",
        "窄屏与桌面布局下固定区域不遮挡内容、不产生错误滚动容器或层叠问题"
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
  "definition_revision": 2,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 固定管理中心各功能页的顶部区域
