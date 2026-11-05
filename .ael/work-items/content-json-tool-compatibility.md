---
{
  "original_request": "假如做一种兼容模式，就是让一些不支持原生tool_call的模型，在content里面返回json，然后在后端识别，转换为tool_call，这样是否可行？先测试，再改。",
  "target": "先用真实模型验证文本 JSON 工具往返可行性，再实现按模型用途显式启用的兼容协议，复用现有 Agent 工具循环、授权和真实回执，完成代码与设置的回归验证。",
  "id": "content-json-tool-compatibility",
  "title": "文本 JSON 工具调用兼容模式",
  "type": "feature",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core 模型调用、工具适配与设置",
    "apps/core-go/internal/ai/agent 和 capability 测试",
    "apps/core-go/internal/httpapi/browser 和模型配置持久化",
    "apps/core-go/internal/migrations",
    "apps/web 模型用途设置",
    "docs/llm-prompt-composition.md"
  ],
  "non_goals": [
    "默认改变所有模型的原生工具调用",
    "通过匹配用户正文或普通回复猜测并执行工具",
    "修改历史聊天、提交图片或发布部署",
    "增加模型输入输出 token 限制"
  ],
  "acceptance": [
    "可针对模型用途显式启用或关闭文本 JSON 工具兼容模式，既有配置默认保留原生模式",
    "兼容模式以明确协议传递当前可用工具，模型输出的调用经现有作用域、参数、授权和幂等链执行并获得真实工具回执，多轮继续正确",
    "原生模式和普通正文 JSON 不被误识别为工具，未知工具、非法参数及重复调用不能绕过现有保护",
    "先用真实 Cydonia 验证不发送原生工具时的 JSON 调用与真实回执续写，并对实现后的模型请求和响应留存脱敏证据",
    "模型设置的读取、保存、快照恢复及相关 Go、Web 验证通过"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": true,
  "ux": false,
  "checks": [
    {
      "id": "compat-go",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core",
        "./internal/capability",
        "./internal/ai/agent",
        "./internal/httpapi/browser",
        "./internal/migrations"
      ],
      "covers": [
        "可针对模型用途显式启用或关闭文本 JSON 工具兼容模式，既有配置默认保留原生模式",
        "兼容模式以明确协议传递当前可用工具，模型输出的调用经现有作用域、参数、授权和幂等链执行并获得真实工具回执，多轮继续正确",
        "原生模式和普通正文 JSON 不被误识别为工具，未知工具、非法参数及重复调用不能绕过现有保护",
        "模型设置的读取、保存、快照恢复及相关 Go、Web 验证通过"
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
        "模型设置的读取、保存、快照恢复及相关 Go、Web 验证通过"
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
        "模型设置的读取、保存、快照恢复及相关 Go、Web 验证通过"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "id": "compat-live",
      "kind": "scenario",
      "command": null,
      "covers": [
        "先用真实 Cydonia 验证不发送原生工具时的 JSON 调用与真实回执续写，并对实现后的模型请求和响应留存脱敏证据"
      ],
      "required": true,
      "timeout_seconds": 0,
      "artifacts": null
    },
    {
      "method": "scenario",
      "id": "compat-settings-ui",
      "kind": "ui",
      "command": null,
      "covers": [
        "可针对模型用途显式启用或关闭文本 JSON 工具兼容模式，既有配置默认保留原生模式",
        "模型设置的读取、保存、快照恢复及相关 Go、Web 验证通过"
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
# 文本 JSON 工具调用兼容模式
