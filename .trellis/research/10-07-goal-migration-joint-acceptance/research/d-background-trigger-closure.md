# 2026-10-08 后台唤醒／反思／目标评估持续占队列修复

## 实际只读诊断

用户运行环境 product.wakeup.enabled=true、interval_seconds=300，generated_concurrency=1。查询最近100次模型调用：Reflection51、WakeUp35、Goal Evaluation6、Cognitive Assessment7、Conversation Summary1。这是模型调用数（含ADK多轮/重试），不是触发次数，不据此伪算精确调用间隔。

旧源码：最后用户聊天后10m/30m，再按配置循环；Reflection quiet=10m，WakeUp每次完成都会创建Reflection；检查/no-op Outcome进入Goal source journal，Reflection又请求Goal Review，因此背景检查可以推动下一轮自身调用。

## 最新用户规则与修复

最后实际用户/助手聊天重置10m WakeUp key，之后10m周期；丢失周期合并为未来一轮，不补发队列洪峰。启动保留已有key/TTL，缺key仅放行一个durable cycle；若同实例有待执行认知，启动不能复活旧WakeUp。读/写settings与Core/Workflow均规范为600s，Web默认和说明同步。

Reflection只在最新聊天30m后一次、有尚未处理真实证据时调用模型。待执行任务按实例合并，但身份由实例＋实际聊天来源确定，旧取消键/旧回调不影响新epoch；同源重放不重开完成任务。Redis镜像PG绝对due，不按模型完成重新推迟30m。真实非聊天结果也可排一次合并的证据处理，但不能早于last-chat+30m。

新认知入队同事务supersede同实例WakeUp/Reflection，提交后Provider marker和Temporal Cancel seam传递取消，最终提交仍查durable status。统一lifecycle→Life锁序。Provider的真实HTTP context取消已验证；Temporal Cancel通过生产接口seam断言，未把这次结果称作真实Temporal Worker重启验证。

静默WakeUp不创建Reflection。噪声窗口无模型推进watermark，跨125条噪声仍能找到真实证据。Goal/Actor/框架检查和控制/no-op不驱动Goal journal；primary receipt携带真实child capability names，不能用聚合回执绕过过滤。真实领域查询/action/message、撤销、Owner reassess和有限source remainder保留。兼容单个legacy outcome与outcomes数组。

唤醒真实发出消息不会取消自身结算；最后聊天clock由其提交结果重置到消息时间+10m。实际行动结果进入证据窗口，周期标签仍不冒充生理事件。Goal Evaluation继续独占目标标准/完成/下一策略；Reflection负责记忆/关系/自我学习，并委托目标writer。没有合并或新增平行业务writer。

## 验证

- 最终全Go包race：d-background-trigger-release-go-all-race.jsonl，1866 PASS events / 1721 leaf PASS / 0 FAIL / 36 SKIP。缺配置的真实Provider/媒体/联合设施SKIP不计通过。
- 启动/取消/Reflection/Goal/原生/发布/工作流门禁：d-background-trigger-final-gate-race.jsonl，310 PASS events / 0 FAIL / 3 SKIP（此前窄门禁；最终以全包release为准）。
- actual WakeUp reply和新增PG/Redis回归：d-background-trigger-own-reply-race.jsonl，9 PASS events / 0 FAIL / 0 SKIP。
- 实际PG/Redis：已有key的TTL保持、missing-key release一次、busy cognition startup拒绝复活、last-chat30m/Redis mirror、完成epoch重放no-op、125静默记录0模型后真实来源1次model、inspection primary不动Goal水位、Provider context取消、late watermark拒绝。强化已有实际WakeUp发布测试：真正带生命周期身份执行，完成一次且消息+10m。
- go vet ./...、go build ./...、gofmt/diff check通过。Web68/68、typecheck/build通过；UI仅默认值/标签变化，未称作真实浏览器验收完成。

失败尝试摘录保留：第一次临时主库未迁移（部分旧测试直接用主库）、单outcome兼容漏读、旧时序断言、新夹具误将合法no_change判成应applied。对应修复后重跑，失败不是通过证据。独立窄复核的startup/新认知竞争P1已修复并新增行为回归。

## 实际部署边界

未修改用户线上设置、业务数据或队列，未部署。实际频率与取消行为只有更新API/Worker/Web后才能验收；之前排队的无新证据任务会走新门禁，真实旧证据不丢弃或伪标完成。原任务、B/C/D和真实验收保持in_progress。隔离PG/Redis为本轮自有设施，收尾只清理这些；秘密URL/password/session和生产原文不入仓库。
