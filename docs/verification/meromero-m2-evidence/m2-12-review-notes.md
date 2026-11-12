# M2-12 direct 关系引用与认知结算

原 T04 要求主链迁移后保留关系等认知影响及可回查的证据。本项只处理一项
具体行为：已有选择规则保留关系时，direct 主 Agent 可以使用 Core 签发的
当前作用域引用作为 `influences`；关系仍按既有上下文规则选择，不强制每轮注入。

在干净提交 `4bf8a52b863315dffd55e7d267038f1376e13182` 加测试文件的
一次性 PostgreSQL 场景中，实际 direct 请求包含关系语义，却没有对应引用；
`m2-12-red.log` 退出 1，伪造引用对照通过。初版修复的受控测试通过，但复核
发现 direct 与 legacy 共用投影，初版也会影响 legacy；随后改为仅对实际 direct
模式增加引用，保留旧模式、takeover、Kev 选择和模型配置。初版结果保留在
`m2-12-initial-*.log`，不作为最终候选的验收。

最终候选源码哈希见 `m2-12-scoped-manifest.json`。在一次性 PostgreSQL 和
受控原生 Provider 下，`m2-12-scoped-focused.log` 的 3 个顶层测试、
`m2-12-scoped-regression.log` 的 12 个顶层回归均退出 0、无 SKIP。场景核对
实际请求中的不透明引用、原始关系 ID 不进入请求、同事务结算及重放、伪造和
跨会话引用拒绝、legacy/takeover 投影与 fact_only 切换。含四处原有草稿的
工作区定向运行见 `m2-12-working-tree.log`。

这些结果使用受控 Provider，不证明真实模型会选择该引用；M2-02 的真实模型
失败和 T04/M2 的其他验收仍未解决。
