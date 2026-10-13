# 第一、第二期执行计划

## 已完成

- [x] 完成全站巡检：16现场问题、1源码线索、22截图与覆盖矩阵。
- [x] 用户明确要求两期全部落地。
- [x] 核对前端规范、治理/配置/诊断契约与补充源码。
- [x] 完成prd/design/implement与context manifests；准备最终方案审阅。
- [x] 用户批准本次扩展后的最终规划摘要。
- [x] 执行task.py start；进入产品实现。

## 批次1：共享基础（R01/R07；F01–F03/F15）

- [x] 统一token语义、Button variants/尺寸/状态，修复低对比度与小触控区。
- [x] 对齐Input/Select/Checkbox/Badge/Dialog，新增必要产品语义包装。
- [x] 字符图标替换Lucide；布局/动效styles与reduced-motion。
- [x] 回归黑色按钮、42px产品按钮和窄屏操作行；不保留冲突CSS补丁。

## 批次2：表单与状态（R02；F04–F06/F16–F17）

- [x] 实现FieldError/Feedback/AsyncState/ConfirmDialog。
- [x] 创建/目标/设置/治理校验与错误隔离；关闭清理，失败保留草稿。
- [x] 中文错误码映射、恢复动作和高级详情；初始化加载ready门禁。
- [x] clearDiagnostics确认从store原生confirm迁移到UI确认。
- [x] 行为测试空白/格式/局部错误/加载/确认取消；保留对话retry原契约。

## 批次3：资料与可读展示（R03/R05；F07–F10/F15）

- [x] session.actorId身份存储生命周期与显示名解析。
- [x] 统一背景主编辑入口、10字段和correct/change，分离Owner/通用CAS adapter。
- [x] 修复异步切对象/空目标/草稿幂等边界；409保留和显式重载。
- [x] 人格资料按常用/生活/原始技术分层，未知字段高级可达。
- [x] 状态词典、时间格式与衣柜/外貌结构化展示。
- [x] 资料draft/fence/SSR和时间provenance现有测试回归。

## 批次4：设置表单化（R06；F11）

- [x] 自治与并发表单化；唤醒固定10分钟+开关；有效保留规则只读。
- [x] 设置开放对象merge编辑字段；封闭schema按后端allowlist序列化。
- [x] 提取工作流parser、导入文件/文本、结构校验、摘要及高级编辑。
- [x] media分支/变体/未知字段无损往返；不新增假模板API。
- [x] 测试数值范围、未知字段、分支配置、裸placeholder与invalid JSON。

## 批次5：诊断统一（R04；F12–F14/F17）

- [x] 共用摘要行、状态、过滤重置与高级条件/日期输入。
- [x] JsonViewer统一折叠/复制/滚动，保留原始Owner内容。
- [x] Workflow唯一运行key、Run/状态/时间；不伪造缺失字段。
- [x] 保留服务器排序/cursor/filter epoch及有旧页时暂停轮询契约。
- [x] diagnostics/time harness与确认路径fixture回归。

## 批次6：全站完成度与验收（R07/AC01–AC08）

- [x] 审查20工作面与深层详情；统一排版、间距、表面层级和微交互。
- [x] 运行`pnpm --filter @fluctlight/web typecheck`。
- [x] 运行`pnpm --filter @fluctlight/web test`。
- [x] 运行`pnpm --filter @fluctlight/web build`。
- [x] 任务自有同源fixture静态服务承载dist；CUA浏览器检查各导航与成功/失败/取消。
- [x] 320/390/768/1440、键盘、触控、对比度与reduced-motion构建规则验证；真实200%与系统偏好启用的工具限制已记录（完成720×500等效重排）。
- [x] 聊天草稿/流式/重试、详情正文独立滚动、服务端刷新恢复回归；真实手机IME与图片延迟滚动未穷举，详见coverage。
- [x] 更新前后截图与F01–F17修复映射，记录真实服务与fixture的边界。
- [x] 只读独立核验、主代理最终验证与必要spec更新；提交前质量检查。

## 验证规则和交付

不使用真实用户数据测试创建/删除/发送/配置保存。现有SSR/script harness验证payload与状态；CUA对production dist fixture做交互和视觉验证。没有现成fixture API server，新增任务自有轻量同源测试服务即可，不引入整套新自动化框架。

命令从仓库根目录执行；预览脚本为`apps/web/scripts/serve-preview.mjs`，实际选择测试API后再启动。无generated DTO变更则不运行不必要的全仓生成；若确需字段调整，按规范更新生成源并运行完整生成与契约测试。

各批风险与回滚点：共享CSS→消费视图→背景adapter→配置序列化→诊断分页。先满足局部行为回归再整合，禁止把测试fixture通过写成真实Provider通过。最终交付代码diff、通过的检查、前后截图和未能在真实服务核验的具体项。

## 最终质量检查（2026-10-13）

全工作区generate/typecheck/test/build及Go browser race/vet/build通过，共127项JS/TS测试。五项最后核验发现已修复；Owner浏览器allowlist同步既有十字段契约。24张虚构样本截图、78条尺寸记录和限制见artifacts/ui-refactor-2026-10-13。

- [x] workflow3.4：用户一次性确认具体提交批次（“按计划提交”）。
- [x] 执行已批准工作提交：6b1d60e。
提交后归档与会话日志由finish-work流程记录；结果以归档task.json及Git记录为准。
