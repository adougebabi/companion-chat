---
{
  "original_request": "生图节点 ratio 支持 1:1、3:2、2:3、16:9、9:16；resolution 支持 512、768、1024、1280、1536、2048，默认 768。resolution 节点使用字符串，填入数字内容并由 JSON 外层双引号形成字符串。",
  "target": "让头像、普通图片和角色设定图按节点实际支持的 ratio 和 resolution 发送请求，保留旧任务冻结的参数及旧工作流兼容性。",
  "id": "comfy-ratio-resolution-presets",
  "title": "适配 ComfyUI 节点比例和分辨率选项",
  "type": "bug",
  "source": "user",
  "scope": [
    "apps/core-go/internal/core 的图片尺寸冻结、视觉身份画幅和 ComfyUI 替换",
    "物理 HTTP 和恢复相关 Go 测试",
    "相关提示词清单和 AEL 视觉、媒体合同"
  ],
  "non_goals": [
    "修改用户真实数据库或生图工作流配置",
    "更改头像主体选择和禁字要求",
    "重新提交已发送的 Provider 任务",
    "引入提示词 token 限额或提交图片"
  ],
  "acceptance": [
    "ratio 支持指定的五个值，头像 1:1，普通图片支持横竖比例，未指定继续默认竖版；新角色参考/设定图使用可选的 2:3 竖版",
    "resolution 可用值为 512、768、1024、1280、1536、2048；默认 768，工作流的字符串占位符发送字符串数字，不发送宽高拼接",
    "既有带旧尺寸格式的任务仍可恢复且未提交时不擅自改写已冻结尺寸，已提交任务继续原任务；旧宽高工作流和固定工作流保持兼容",
    "视觉角色卡生成及审核兼容新的竖版和旧 3:4 任务，相关真实请求回归及 Go Core 测试通过"
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
      "id": "core-tests",
      "kind": "command",
      "command": [
        "go",
        "-C",
        "apps/core-go",
        "test",
        "./internal/core",
        "-count=1"
      ],
      "covers": [
        "ratio 支持指定的五个值，头像 1:1，普通图片支持横竖比例，未指定继续默认竖版；新角色参考/设定图使用可选的 2:3 竖版",
        "resolution 可用值为 512、768、1024、1280、1536、2048；默认 768，工作流的字符串占位符发送字符串数字，不发送宽高拼接",
        "既有带旧尺寸格式的任务仍可恢复且未提交时不擅自改写已冻结尺寸，已提交任务继续原任务；旧宽高工作流和固定工作流保持兼容",
        "视觉角色卡生成及审核兼容新的竖版和旧 3:4 任务，相关真实请求回归及 Go Core 测试通过"
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
  "feedback_of": "avatar-creative-prompt-image-dimensions",
  "definition_revision": 1,
  "max_retries": 3,
  "max_seconds": 0
}
---
# 适配 ComfyUI 节点比例和分辨率选项
