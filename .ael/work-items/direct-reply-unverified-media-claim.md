---
{
  "original_request": "用户原始 T04/T06 明确要求完成声明及附件通过对应核验后才正式发布，正文与真实动作、交付状态一致。T04 隔离 PostgreSQL、受控 Provider 场景中，主 Agent 只调用 conversation.reply，合成正文为“我已经把照片发给你了。”，未调用媒体工具；当前 direct 实际提交了一条附件为空的助手消息，turn_error 为 nil。失败日志及可重放探针见 docs/verification/meromero-t06-direct-media-claim-red.txt 和同目录 probe.go.txt。当前未证明真实模型一定产生该正文。",
  "target": "补齐 direct 回复的动作与媒体正文一致性核验，在没有真实交付证据时阻止或纠正错误完成声明，沿用统一发布、原生工具和真实回执。",
  "id": "direct-reply-unverified-media-claim",
  "title": "Bug：direct 回复可在无附件时声称图片已发送",
  "type": "bug",
  "source": "user",
  "scope": [
    "direct 主回复的未发布候选、真实动作与媒体状态、语义核验、修复或终止，以及相关旧路径回归"
  ],
  "non_goals": [
    "不只依赖正则或模型自报一致",
    "不固定引入 Qwen 审核或强迫角色接受动作",
    "不伪造工具、媒体、交付、看图或业务数据",
    "不提交图片"
  ],
  "acceptance": [
    "隔离环境证明无媒体调用或失败/排队/未交付时，错误已发送声明不会正式发布",
    "真实附件与正文声明一致，accepted、ready、delivered 和质量状态分开",
    "有界修复保留候选、核验与实际回执，失败不得伪造成功；动作和发布重放幂等",
    "自然拒绝、未来意愿和正常纯文本交流不被统一模板压制"
  ],
  "priority": 1,
  "dependencies": null,
  "parent": "meromero-t06-delivery-consistency",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "direct-media-claim-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "隔离环境证明无媒体调用或失败/排队/未交付时，错误已发送声明不会正式发布",
        "真实附件与正文声明一致，accepted、ready、delivered 和质量状态分开",
        "有界修复保留候选、核验与实际回执，失败不得伪造成功；动作和发布重放幂等",
        "自然拒绝、未来意愿和正常纯文本交流不被统一模板压制"
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
# Bug：direct 回复可在无附件时声称图片已发送
