# 规划收敛检查

日期：2026-10-13；状态：等待最新最终摘要的用户确认，全部任务仍为 planning。

## 已完成

- 原始需求复制字节一致，SHA256 `9865d0ba21303081339c79714a91fadcb21cfa95c3371ba551e8931041d06fb7`。
- 已 fetch origin/master，功能分支 `codex/fact-only-reply-vision` 的 HEAD 为 `e36b3be8b2efb91e02fbed2771352f49557a60df`；本地 master `6e11490c381fbb61cad73dda686b35846eda2f06` 保留。功能分支清除了自动指向 origin/master 的 upstream，未来只显式 push 功能分支。
- 四个探子分别盘点代码/测试/配置，主代理抽查 reply Definition/DirectTx、ExecuteTool Prepare、原生合同、Provider binding、CI发布门与master前置修复出处，未外包奠基文档或产品实现。
- 父任务及六个子任务均有非空 PRD、design、implement、implement/check JSONL，无 TBD/TODO。
- 六条 parent/child 关系、branch/base_branch 元数据均已核查；T01–T20 原文验收行全部保留。
- 七个 `task.py validate` 全部通过；这仅验证上下文清单引用，不代表业务测试通过。
- 产品代码与既有任务文件没有修改，未运行 `task.py start`，未提交/push/dispatch/部署/合并。

## 上下文加载提醒

现 context_injection.max_file_bytes 为 32768，部分契约（Provider、Structured Turn、Workflow、Autonomy、Persona、Diagnostics）超过该限制，validate 有截断警告。清单合法不等于契约全文已加载。实施前主代理按用户要求完整读适用奠基契约；后续核验代理只接受聚焦问题与具体出处，不能根据注入截断声称完整合规。若适用上层流程需要 implement/check 子代理，dispatch 必须明确全文 child-side pull fallback，分段读直到文件末尾，不能依靠缺尾的自动注入。不会为了本任务扩大平台上下文配置或重写Trellis。

`.trellis/spec/guides/index.md` 在当前仓库缺失；本轮按现有 backend/frontend index 与实际契约盘点，没有补写不实指南。

## 收敛结论

目标、范围、非目标、观察型验收、六项顺序和权威边界已经明确；没有必须让用户补充的产品取舍。Provider/draft/vision实现细节可在不改MVP的前提下按实际接口收敛。唯一阶段门是用户明确批准最新规划摘要。

## 分层证据状态

| 层级 | 本轮状态 |
| --- | --- |
| 仓库/基线/调用链盘点 | 已执行，代码锚点在 research/ |
| Context7 Eino/LM Studio 文档核对 | 已执行，版本覆盖限制已记录 |
| 任务关联、规划产物、原文与验收矩阵检查 | 已执行，通过（注入截断警告见上） |
| 产品实现 | 待规划批准后开始 |
| 单测/真实registry+ADK+发送集成 | 未执行 |
| 真实模型10–20次/三条10轮fixture | 未执行，现无已确认配置及地址 |
| GitHub Actions/registry双镜像发布 | 未执行，本机gh不可用；未来可用授权connector/其它方式单独核查 |
| Docker/Compose健康及测试环境回滚 | 未执行，Docker daemon当前不可用 |
| 稳定digest、生产DB版本、私有配置备份 | 未确认，不填猜测值 |

批准后第一步启动 FO-A（10-13-fact-only-release），不误从最后创建的验收子任务开始。六项完成后停在功能分支等待用户真实测试，不自动合master。
