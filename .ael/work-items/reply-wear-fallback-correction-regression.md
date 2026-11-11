---
{
  "original_request": "用户要求非阻断问题先登记，不偏移原任务。T12 在提交 42b43abf 的隔离 PostgreSQL 回归中，TestR6PrivateChatDiscoveryWearReceiptAndWriter/wear_unavailable_fallback_repair 的流式与非流式子场景失败：工具返回 wardrobe_item_unavailable 后测试记录 unexpected additional cognition request，最终受控 Provider 返回 400；同样两个场景在起始提交 ffa808bc81cc4d800376432216f3ea679da8de30 的独立源码与同一隔离数据库环境均通过。旧夹具使用 HTTP 400 触发 fallback，与当前将 4xx 视为不可回退的实现不一致；临时改为可回退的 503 后仍失败，实验已撤回。单独的受控测试确认 400 不调用备用模型。尚未确定是测试合同、模型绑定、运行恢复还是生产行为回归，不得根据旧测试要求恢复不安全的回退。日志 /private/tmp/fluctlight-t12-head-capture-wear.log、/private/tmp/fluctlight-t12-baseline-wear-fallback.log、/private/tmp/fluctlight-t12-wear-fallback-updated.log。",
  "target": "后续确定换衣不可用与明确技术故障下 Writer 回退和事实纠正的合同与失败原因，并修复实现或过期夹具。",
  "id": "reply-wear-fallback-correction-regression",
  "title": "Bug：换衣不可用后的 Writer 回退纠错回归失败",
  "type": "bug",
  "source": "user",
  "scope": [
    "私聊衣柜不可用回执、显式技术故障回退、事实一致性纠正和相关流式/非流式隔离回归"
  ],
  "non_goals": [
    "当前 T12 只登记和保留失败，不调整生产逻辑",
    "不得因业务拒绝、工具条件不满足或输出一致性失败而换模型重新决定",
    "不连接生产数据库或提交图片"
  ],
  "acceptance": [
    "确认原测试失败的具体原因并区分过期夹具与生产回归",
    "HTTP 400、业务拒绝和输出一致性失败不能触发不合规的备用模型调用",
    "仅在明确技术故障和显式策略允许时覆盖备用模型及其纠正链，保持真实工具回执和未穿衣事实",
    "隔离 PostgreSQL 中流式和非流式场景均有可复核结果"
  ],
  "priority": 2,
  "dependencies": null,
  "parent": "meromero-t12-final-acceptance",
  "goal": "verified",
  "environment": "",
  "preset": "standard",
  "risk": null,
  "ui": false,
  "ux": false,
  "checks": [
    {
      "method": "scenario",
      "id": "wear-fallback-correction-scenario",
      "kind": "scenario",
      "command": null,
      "covers": [
        "确认原测试失败的具体原因并区分过期夹具与生产回归",
        "HTTP 400、业务拒绝和输出一致性失败不能触发不合规的备用模型调用",
        "仅在明确技术故障和显式策略允许时覆盖备用模型及其纠正链，保持真实工具回执和未穿衣事实",
        "隔离 PostgreSQL 中流式和非流式场景均有可复核结果"
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
# Bug：换衣不可用后的 Writer 回退纠错回归失败
