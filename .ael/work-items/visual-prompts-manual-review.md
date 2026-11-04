---
{
  "original_request": "形象参考图、角色设定图、显示头像这3个的现在的prompt是怎样的。按照我测试的结果，应该只有角色设定图？只有视觉身份有东西，而且需要审核（我未开始视觉大模型），人工审核流程是卡住的，点审核过不了。",
  "target": "核对三种图片的当前提示词，并实现关闭视觉模型时的直观人工通过/不通过评审；不通过的真实证据用于下一次生成，模型评审保持同样的结果与证据语义。",
  "id": "visual-prompts-manual-review",
  "title": "核对视觉图片提示词并排查人工审核",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/web/src/react-app/shared/VisualWorkspace.jsx",
    "apps/web/src/react-app/shared/WorkspaceCommandDialog.jsx",
    "apps/web/src/styles/replica-chat.css",
    "apps/web/src/styles/replica-media.css",
    "apps/web/test/demo-replica-media.test.mjs",
    "packages/browser-client",
    "apps/core-go/internal/httpapi/browser",
    "apps/core-go/internal/core/visual_identity*",
    "apps/core-go/internal/core/owner_visual_review.go",
    "视觉身份提示词组成清单和适用规范"
  ],
  "non_goals": [
    "自动接受用户真实候选图",
    "虚构人工评分、观察或理由",
    "提交图片、合并或发布"
  ],
  "acceptance": [
    "准确说明三种图片实际提示词、动态输入和触发顺序",
    "人工审核用 Radio Group 明确选择通过或不通过，通过无需填写评分、观察或理由",
    "不通过时收集真实问题并用于下一次生成；模型评审失败的真实观察与修改建议同样保留并用于生成",
    "关闭视觉模型保留待审核候选；人工与模型来源、版本围栏、幂等和恢复语义保持正确",
    "错误以 Sonner 即时提示，并保留可理解的页面反馈；相关前后端与真实页面验证通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "id": "prompt-audit",
      "kind": "model",
      "command": null,
      "covers": [
        "准确说明三种图片实际提示词、动态输入和触发顺序"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "web-test",
      "kind": "command",
      "command": [
        "pnpm",
        "--filter",
        "@fluctlight/web",
        "test"
      ],
      "covers": [
        "人工审核用 Radio Group 明确选择通过或不通过，通过无需填写评分、观察或理由",
        "错误以 Sonner 即时提示，并保留可理解的页面反馈；相关前后端与真实页面验证通过"
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
        "错误以 Sonner 即时提示，并保留可理解的页面反馈；相关前后端与真实页面验证通过"
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
        "错误以 Sonner 即时提示，并保留可理解的页面反馈；相关前后端与真实页面验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "manual-review-core",
      "kind": "scenario",
      "command": null,
      "covers": [
        "人工审核用 Radio Group 明确选择通过或不通过，通过无需填写评分、观察或理由",
        "不通过时收集真实问题并用于下一次生成；模型评审失败的真实观察与修改建议同样保留并用于生成",
        "关闭视觉模型保留待审核候选；人工与模型来源、版本围栏、幂等和恢复语义保持正确"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "manual-review-ui",
      "kind": "scenario",
      "command": null,
      "covers": [
        "人工审核用 Radio Group 明确选择通过或不通过，通过无需填写评分、观察或理由",
        "不通过时收集真实问题并用于下一次生成；模型评审失败的真实观察与修改建议同样保留并用于生成",
        "错误以 Sonner 即时提示，并保留可理解的页面反馈；相关前后端与真实页面验证通过"
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
# 核对视觉图片提示词并排查人工审核
