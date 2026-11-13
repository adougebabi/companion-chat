---
{
  "original_request": "M2/T04 要求正式主对话的真实工具动作、持久性及恢复不重复。静态核查发现流式领取过期后 attempt 变化可能使已提交工具动作获得新操作身份；先用隔离环境复现，再处理该直接主链缺口。",
  "target": "避免同一 direct 流式对话在已提交工具动作后中断、过期恢复时重复执行该动作。",
  "id": "meromero-m2-18-direct-claim-recovery",
  "title": "M2-18 direct 流式领取过期后的工具动作防重",
  "type": "feature",
  "source": "verification",
  "scope": [
    "正式 direct 流式对话领取、同轮工具已提交事实保护与相关隔离回归"
  ],
  "non_goals": [
    "不处理独立多 LoRA 工作流兼容工单或其他非阻断旁支",
    "不改变默认模型选择、提示词、采样、token 规则、fact_only 开关或数据库结构",
    "不连接生产数据库，不修改四处原有草稿；受控 Provider 不冒充真实模型"
  ],
  "acceptance": [
    "正式流式 direct 主链中，真实提交一个工具动作后模拟进程消失及过期领取，同轮恢复不得再次提交该动作，也不得把未发生的回复伪称已完成",
    "已提交动作继续可核对，采用现行部分提交保护或有真实回执支持的安全恢复；不遗漏或回滚已经提交的业务事实",
    "没有已提交写动作的过期请求仍可恢复，已完成消息重放、正常新请求及旧模式相关行为不回退",
    "隔离 PostgreSQL 及受控 Provider 中先保留真实红测，再执行修复后主场景和直接相关回归；真实模型验收单独保留"
  ],
  "priority": 0,
  "dependencies": null,
  "parent": "meromero-t04-single-main-chat",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "direct-claim-recovery",
      "kind": "scenario",
      "command": null,
      "covers": [
        "正式流式 direct 主链中，真实提交一个工具动作后模拟进程消失及过期领取，同轮恢复不得再次提交该动作，也不得把未发生的回复伪称已完成",
        "已提交动作继续可核对，采用现行部分提交保护或有真实回执支持的安全恢复；不遗漏或回滚已经提交的业务事实",
        "没有已提交写动作的过期请求仍可恢复，已完成消息重放、正常新请求及旧模式相关行为不回退",
        "隔离 PostgreSQL 及受控 Provider 中先保留真实红测，再执行修复后主场景和直接相关回归；真实模型验收单独保留"
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
# M2-18 direct 流式领取过期后的工具动作防重
