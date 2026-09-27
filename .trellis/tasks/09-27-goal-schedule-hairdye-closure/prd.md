# 目标意图驱动日程与染发行动闭环

## Goal

摇光对“去染发”等未来虚拟生活行动先形成持久 Goal/Intention，并把行动安排进权威日程；到预约时间才开始活动，确认完成后才改变当前发色。私聊叙述应与日程、活动、发色的实际状态一致。

## Confirmed gaps

- Conversation 只有 `schedule.replan`，它要求已有当天 accepted 日程；Goal/Intention 与日程事项没有持久关联。
- `agency.intention_due` 被 Worker 当普通聊天 fact 处理，绕过 Native Cognition。
- 日程事项到点只影响 Life Context，不会启动能力；`life.activity.start` 目前立即开始，并无 `hair_dye` 类型。
- `appearance.style` 可立即写入任意发型文字，可能让“染成某色”以当前发型的形式出现；普通私聊提示没有明确区分计划、进行中、已完成。

## Requirements

- R1：为可执行的未来虚拟活动提供单一受控计划入口。它在一个业务提交中建立或复用 Goal/Intention、接受含明确关联的日程版本、设置与日程开始时间相同的 typed Intention trigger；失败不得留下半个承诺。现有当前日程缺失时须显式待安排，不得声称已经预约。
- R2：已接受日程的关联事项到点后，持久工作流验证当前日程版本、Intention 状态与时间，再启动相应活动。日程文本本身不得直接执行 Tool；重放和重启不得重复开始。
- R3：`hair_dye` 是独立活动。安排或开始均不得改变当前 `hair_color`；到最早完成时间后获得独立结果，只有真实 `completed` 且结果颜色与目标一致才改变发色。失败或延期保留旧发色并正确结算意图。
- R4：修复 `agency.intention_due` 的 Worker 路由，使无绑定日程的其他 typed Intention 正常进入 Native Cognition。
- R5：私聊只依据权威日程和真实活动状态声称“计划染发”“正在染发”或“染完了”；未开始的计划不能变成当前生活场景。直接临时发型工具不能承担染发的当前发色变更。
- R6：日程重排/取消、意图暂停或取消、错过时间、Provider 错误与重复触发都必须显式处理，不能继续执行旧版本计划或伪造完成。

## Out of scope

- 真实理发店预约、支付、外部导航或线下结果确认；这里的行动是现有虚拟生活机制。
- 从日程自由文本推断任何可执行能力。只有带受控行动类型与关联意图的事项可自动启动。
- 将所有现有任意能力一并改造成定时行动；本任务打通已支持的虚拟活动类型，并以染发为完整验收用例。

## Acceptance criteria

- [x] A1：从私聊的未来染发请求产生 Goal、qualified Intention 与同一 accepted 日程版本中的关联事项；当前发色不变，聊天称其为计划而非正在进行。
- [x] A2：日程开始前，包括工作流重启和重复投递，均无染发活动；到点后恰有一个 `hair_dye` 活动、Intention 为 `in_progress`，日程与当前活动不冲突。
- [x] A3：活动最早完成时间前发色不变；完成结果为 `completed` 且目标颜色匹配时更新发色和 Intention/Goal；失败或延期不改变发色。
- [x] A4：日程重新安排、取消、意图暂停/取消或旧版本触发不能启动过期行动；已有其他 `agency.intention_due` fact 走 Native Cognition。
- [x] A5：私聊上下文、能力 schema、迁移、生成客户端、相关测试与 Trellis 规范同步；独立 PostgreSQL 与 Workflow 回归通过。
