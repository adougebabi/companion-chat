---
{
  "original_request": "M2/T05 原始要求 Moment 支持可选媒体，Image 是可选依赖；插件实际入口、异步结果和统一交付须可用，普通私聊通过配置的主模型和原生工具运行。",
  "target": "让正式 direct 私聊能显式为本轮已发布的动态请求图片，并通过现有 Image 与统一交付路径把完成的图片绑定到该动态。",
  "id": "meromero-m2-08-moment-image-target",
  "title": "M2-08 私聊中动态附图的正式目标绑定",
  "type": "feature",
  "source": "user",
  "scope": [
    "模型可理解且有界的动态附图目标声明",
    "本轮已提交 Moment 工具结果到 Image 正式目标的授权绑定",
    "独立 Image 工具、插件停用、异步交付及重放回归"
  ],
  "non_goals": [
    "不建设第二套插件或 Runtime，不隐式把普通会话图片改投动态",
    "不暴露或接受任意未授权的内部 Moment ID，不依赖模型编造资源引用",
    "不修改用户保留的四处未暂存草稿、fact_only 开关、默认模型配置或采样",
    "不改变图片提交不确定时暂停并等待人工核对的规则",
    "不连接真实业务库或把受控服务冒充真实模型和渲染"
  ],
  "acceptance": [
    "默认 direct 私聊可在前一轮真实 Moment 发布回执后显式请求动态附图；正式媒体任务目标是该动态而非私聊",
    "受控渲染完成后图片、引用和动态附件各交付一次；待处理及失败不伪称已附图",
    "没有本轮已提交动态、同批未完成或非授权目标时不创建错误媒体任务；未显式选动态的普通图片仍投原目标",
    "Image 停用时纯文字动态仍可用且不创建新图片；独立工具使用同一目标授权和幂等规则",
    "受控集成与直接受影响回归通过，不据此宣称真实模型、真实生图或整个 M2 完成"
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
      "id": "m2-08-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "默认 direct 私聊可在前一轮真实 Moment 发布回执后显式请求动态附图；正式媒体任务目标是该动态而非私聊",
        "受控渲染完成后图片、引用和动态附件各交付一次；待处理及失败不伪称已附图",
        "没有本轮已提交动态、同批未完成或非授权目标时不创建错误媒体任务；未显式选动态的普通图片仍投原目标",
        "Image 停用时纯文字动态仍可用且不创建新图片；独立工具使用同一目标授权和幂等规则",
        "受控集成与直接受影响回归通过，不据此宣称真实模型、真实生图或整个 M2 完成"
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
# M2-08 私聊中动态附图的正式目标绑定
