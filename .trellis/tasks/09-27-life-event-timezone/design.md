# 设计

## 时间来源

Core 保留 UTC instant 作为排序与权威时间，另保存作者发送时区快照供显示：IANA zone 表示地区规则，发送当时 UTC offset 冻结历史墙钟。用户 turn 从浏览器首次发送时取 zone/offset，并在离线排队、重试和幂等回放中复用；Core 校验并写入 message，流式与历史 DTO 都返回。摇光回复在提交时取该人格生效时区及当时 offset，而不是继承用户时区。旧消息字段为 null，不伪造 provenance；UI 用明确标注的 legacy 显示规则。

初始化分析已有 `identity.timezone`；若描述未提取到时区，使用激活时捕获并经服务端校验的初始化设备 IANA 时区填充，只有两者都缺时才用公开且稳定的最终默认值。显式 Foundation 修改覆盖该值，并继续走现有 timezone-change invalidation。各处统一调用同一有效时区 resolver；`EnsureCurrentDaySchedule` 的 ready/pending/generated 结果都回 timezone，不允许 workflow 静默退化为固定 24 小时。对人格相关 `datetime-local`，用人格时区解析并在请求中携带带 offset 的 RFC3339 instant；诊断系统时间按 Owner 查看设备时区显示并标注。

## 通用事件生命周期

临时活动不再只写独立 run：开始时冻结同一个活动语义、scene/location 与有效期，建立受 Event authority 控制的当前状态；默认结束边界不跨下一个 accepted schedule item，除非有明确延期/重排决定。活动 run 与事件互相关联。明确延期更新两者的有效期，业务效果单独等结果确认。到期无延期时，投影立即忽略该事件及其 run；durable timer/收敛任务把 run 写成可追溯的终态，即使 Worker 延迟也不会让旧 run 重新成为“当前”。

虚拟活动结果允许“活动已结束、没有购入/外观变化”。`completed` 与 `acquired_item` 不再强绑定，只有明确的结果字段可引起衣橱/外观写入。绝不因日程计划到期而创造购入物。旧未关联 Event 的活动 run 通过兼容读取规则/一次性收敛处理，避免升级后永久显示为当前。

## 兼容与风险

新增消息时区字段、必要的 event/run 关联或终态采用向后兼容迁移。Core/API/BFF/browser-client/Pinia/UI 必须同批更新；未提供时区的旧 API 客户端仍可发送，使用有界 fallback 并标明未知。现有衣橱与前端详情有进入任务前的未提交改动，实施时保留其内容。事件完成与购买成功拆分会改变现有虚拟购物结果 schema，须同步其生产者与消费者。
