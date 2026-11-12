---
{
  "original_request": "M2/T05 要求无人物画面不注入角色外貌与穿着；沿用已有图片工作流校验，不能让固定正向条件覆盖冻结的图片事实。",
  "target": "让 SDXL 正向条件的 text_g/text_l 与现有正向文本接受同一权威校验，阻止固定人物或衣着注入。",
  "id": "meromero-m2-06-sdxl-positive-guard",
  "title": "M2-06 无人物图片的 SDXL 正向文本边界",
  "type": "bug",
  "source": "user",
  "scope": [
    "现有图片正向条件图遍历与 SDXL 文本字段",
    "隔离 PostgreSQL、受控模型及渲染请求边界回归"
  ],
  "non_goals": [
    "不修改用户保留的 media_capture_plan.go 和 media_prompt_branches.go 草稿",
    "不更改 Prompt、模型选择、采样、token 限额或负向质量配置",
    "不连接真实业务库，不声称受控 renderer 是真实 ComfyUI",
    "不重构图片插件或处理其他无关缺陷"
  ],
  "acceptance": [
    "无人物场景中，SDXL 正向 text_g 或 text_l 固定人物/衣着时在 renderer 提交前拒绝",
    "正向 text_g/text_l 均使用冻结提示词或安全空值时保持可用，负向质量文本不被误拦",
    "校验进入正式 ProcessMediaIntent，隔离库验证拒绝和正常对照的实际请求边界",
    "保留红绿证据，其他图片主体、视角和真实渲染验收不因此标完成"
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
      "id": "m2-06-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "无人物场景中，SDXL 正向 text_g 或 text_l 固定人物/衣着时在 renderer 提交前拒绝",
        "正向 text_g/text_l 均使用冻结提示词或安全空值时保持可用，负向质量文本不被误拦",
        "校验进入正式 ProcessMediaIntent，隔离库验证拒绝和正常对照的实际请求边界",
        "保留红绿证据，其他图片主体、视角和真实渲染验收不因此标完成"
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
# M2-06 无人物图片的 SDXL 正向文本边界
