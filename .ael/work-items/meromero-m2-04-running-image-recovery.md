---
{
  "original_request": "M2/T05-T06 要求图片已受理任务具备异步状态、恢复和幂等交付；插件停用不能丢失已有任务或重复交付。",
  "target": "验证已受理图片在 renderer job ID 持久化、资产尚未 ready 时跨进程恢复并只交付一次。",
  "id": "meromero-m2-04-running-image-recovery",
  "title": "M2-04 图片任务已保存 Job ID 后跨进程恢复",
  "type": "feature",
  "source": "user",
  "scope": [
    "已持久化 renderer job ID 的未完成图片 intent 及重放",
    "一次性 PostgreSQL 与受控 HTTP renderer、对象存储和模型夹具",
    "正式 Image 调用、关闭 Image 后原任务恢复和新请求拒绝"
  ],
  "non_goals": [
    "不宣称验证真实 Temporal、ComfyUI、对象存储或模型质量",
    "不覆盖 renderer 已接单但 job ID 尚未落库的崩溃窗口",
    "不修改 Prompt、采样、token 上限或用户已有草稿",
    "不重构图片插件或引入新 Runtime"
  ],
  "acceptance": [
    "正式 Image 调用创建 pending intent，renderer 返回 job ID 后失败时该 ID 和未 ready 状态被保存",
    "关闭 Image 后由新的操作系统进程恢复相同任务，不再次提交 renderer，完成持久状态及受控交付",
    "重复执行只保留一份资产、会话附件消息和资源引用，新的 Image 请求仍被拒绝",
    "保留真实测试命令和退出码，明确受控服务及未覆盖的崩溃窗口"
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
      "id": "m2-04-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "正式 Image 调用创建 pending intent，renderer 返回 job ID 后失败时该 ID 和未 ready 状态被保存",
        "关闭 Image 后由新的操作系统进程恢复相同任务，不再次提交 renderer，完成持久状态及受控交付",
        "重复执行只保留一份资产、会话附件消息和资源引用，新的 Image 请求仍被拒绝",
        "保留真实测试命令和退出码，明确受控服务及未覆盖的崩溃窗口"
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
# M2-04 图片任务已保存 Job ID 后跨进程恢复
