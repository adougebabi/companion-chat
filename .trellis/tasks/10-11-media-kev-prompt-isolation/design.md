# 技术设计

## 边界与现有链路
Conversation/WakeUp/直接 Tool → ExecuteTool → imageGenerateCapability.Prepare/ExecuteDirectTx → media_intents.prompt 冻结 → Worker media.go → runMediaPromptTaskResult → Comfy workflow/reference/placeholder → /prompt → 原有 asset/返回。
现有 current_capture 路径按 appearance 存在触发，不代表本人入镜；本轮必须改变上游 Context 依赖，不能只在发送给 LLM 前删字段。完整入口证据见 research/entry-map.md。

## 契约和顺序
在现有媒体 DTO/冻结 concept 内增加版本化 intent、capture_plan、subject scope、field_sources、render policy，不维护平行任务系统。保留原 intent/capture 原文与明确要求来源。
程序只校验结构、候选身份与兼容组合；自然语言主体/约束由已有模型语义层或 Kev 识别，不采用 contains/regex 分类。已有可信结构字段直接锁定；自然语言不得假装已有可信约束。
入镜计划完成之前媒体能力不请求 SlotVisualIdentity/SlotAppearance/SlotCurrentState；修订 tool_contract RequiredContext 与 capability resolver 的资料加载时序。本人确认后按可见范围读授权有效快照；未知范围使用中性输入及同一模型基础层的有界解析/补全。普通明确意图与 Kev 成功路径不多做一次主题规划。
none/no/partial/unknown 计划分别决定白名单和身份参考策略，不能用 primary_subject 判断是否有人。多主体按已有候选 ID 与来源分区。快照版本与 retry/replay 保持一致。

## Kev
复用 decision.Service、原 audit/spool、队列、deadline、配置版本复查。增加三个媒体 point 和媒体总开关；保留七点旧配置读取兼容和关闭语义。
扩展 choice 响应校验为“该问题 criteria 的有限候选”，既有 yes/no/unclear 原路径保留。原始响应和分布先保存再采用；未知、冲突及不兼容补充单独记录。通常先主体/入镜批量，再合并适用拍法/视图批量；无人物不问自拍，明确字段不重问，不新增健康探测或预热。

## 规范化与渲染
统一通过现有 FormalAgentMediaPrompt/模型基础层运行专用隔离编译任务，输出中文正文及最小内部声明。不把当前人物的封闭姿态计划当作全部媒体的正文表示；修改 current_capture 选择/渲染，移除公共 selfie 和第一人称全身回退，以真正画面计划路由。
原意与显式约束锁定，摄影参数允许 unspecified。保留 VisualIdentity 自我创建流程边界，其 seed/character sheet 不套无人物 scene 分支。
校验来源清单、结构、候选ID、占位符和典型高精度污染，LLM自述不能作为语义正确证明。已检测错误只执行一次同模型规范化修复，修复不重问 Kev、不生图；再失败终止提交。

## 最终工作流
media.go 在实际 /prompt 前对执行图正向条件、LoadImage/reference、身份LoRA/weight及分支策略核验。深复制 workflow/消息/列表，避免原地清理共享对象。风格LoRA不等于身份LoRA，身份语义需配置或明确节点契约，不能靠所有LoRA清空。
如现有 workflow 强制身份输入，优先选已有中性变体，否则最小配置/节点分支；不可把缺身份参数当作已关闭身份条件。未能安全隔离时返回明确配置错误，不提交已知污染。

## 兼容、版本与回退
新冻结 concept/模板/问题/渲染策略均标版本，缓存及复用包含原意、约束、入镜计划、状态版本、模板和配置版本。旧持久化任务明确解析兼容，不静默把旧自拍 prompt 复用到新无人物请求；已完成任务不重生成。
媒体指令目前是代码常量，settings 中 media.comfyui 是运行工作流。实施前保存现有默认和本地配置的脱敏差异/可恢复安全备份，定制配置冲突显式诊断。配置写入遵循 Owner API/原加密机制，凭据不落报告。
与旧 media-prompt/visual-identity specs 的默认自拍、framing可改写及第一人称full_body恢复有冲突，本任务验收后以新分支契约更新这些条款，保留合法人物能力。回退代码/配置版本不得回滚业务事实或已生成artifact。

## 验证边界
A层 stub 契约是工程保证，B真实Kev、C真实规范化文本、D真实隔离图片分别证明对应层；服务不可访问则交付可执行命令和未执行原因。无需新增常驻审查模型或视觉闭环。
