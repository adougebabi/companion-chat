---
{
  "original_request": "用户要求：不要偏移原来的任务设定。任务外、非阻断性的 bug 或调整先创建 bug 任务，不在当前任务修改。T02 期间发现正式设置页面将 Provider 地址与密钥作为两个独立命令保存，两次提交之间请求可能读到混合配置。 已停止的未完成实验保存在分支 codex/t02-provider-atomic-wip 的提交 f644c8bc，仅供后续参考，未合入主线；其密钥单独更新的绑定行为及契约大小限制仍需复核。",
  "target": "后续单独解决同一次服务地址与凭据变更的保存一致性，保留独立密钥轮换及已配置模型用途的合理行为。",
  "id": "provider-endpoint-credential-atomic-save",
  "title": "Bug：服务地址与凭据分开保存可能产生中间配置",
  "type": "bug",
  "source": "user",
  "scope": [
    "Provider 地址与密钥的联动保存、配置命令与正式设置页面",
    "相关失败恢复、版本冲突、凭据脱敏和回归"
  ],
  "non_goals": [
    "不连接生产环境或使用真实凭据",
    "不改模型输入输出 token 限制",
    "不提交图片或绕过提交钩子"
  ],
  "acceptance": [
    "同一次地址和凭据变更原子提交，失败或版本冲突不留下部分配置",
    "独立密钥轮换及其既有配置用途保持可用，密钥不出现在回执、日志或浏览器持久存储",
    "隔离数据库与受控页面验证成功、回滚、重放和保存中修改"
  ],
  "priority": 2,
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
      "method": "scenario",
      "id": "provider-atomic-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "同一次地址和凭据变更原子提交，失败或版本冲突不留下部分配置",
        "独立密钥轮换及其既有配置用途保持可用，密钥不出现在回执、日志或浏览器持久存储",
        "隔离数据库与受控页面验证成功、回滚、重放和保存中修改"
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
# Bug：服务地址与凭据分开保存可能产生中间配置
