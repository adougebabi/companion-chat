# 摇光媒体 Kev 与分支上下文隔离实施报告

日期：2026-10-11。分支：codex/media-kev-prompt-isolation。任务：10-11-media-kev-prompt-isolation。

## 交付结论与验证边界
已实现媒体分支、惰性人物资料装配、Kev 三点、多选协议、独立中文规范化、有界修复、最终工作流检查、配置UI/操作入口和测试产物。真实服务验证及目标环境配置启用按用户补充由用户完成；本轮未启动本地模拟应用环境，未执行独立探针、真实Kev/LLM/ComfyUI调用或社交发布。

工程用例验证程序边界，不证明模型能正确理解每段中文，也不证明像素构图。复杂自由文本的一致性仍需真实规范化输出与图片抽查，不能用模型声明字段替代实际效果。

## 已确认原因及修改证据
- 原 builtin_capabilities.go 的 imageGenerateCapability.Prepare 无条件冻结 visual_identity/current_life/appearance/current_state，原 Definition/RequiredContext 也固定四Slot。现在提前声明零必需Slot，先形成计划，再读取适用资料；请求和准备快照都做投影，none不含本人数据。
- 原 current_capture.go 的 hasCurrentCapture 依据 appearance存在进入人物路径，currentCaptureCameraPhrases 默认selfie，renderCurrentCapturePrompt 追加人物身份、着装和当前场景。普通任务现在使用 media.capture.v1 和独立中文规范化；旧模板只留作历史辅助，未提交旧任务先升级，缺原意的旧人物请求明确拒绝。
- 原 model_tasks.go 的人物路径覆盖普通指令，模型只能返回姿态计划，正文由Core生成。新路径返回中文正文与内部声明，保留原意、原capture描述、明确人数和入镜约束。
- 原 decision.go 只有七点和yes/no/unclear解析。现在追加 media.subject_route/media.capture_plan/media.view_plan，按每题criteria验证多选，保留二元行为；第二阶段拍法与视图可同批并各自留痕/撤销。
- 原数据库0056的CHECK限制七点。新增0060_media_kev从0059升级约束；旧0056 SQL未改，审计/配置不删除。
- 普通参考和身份参考分别替换，避免猫/商品图误填本人身份通道。最终检查固定正向文字、固定身份图、人物适配器、未分类LoRA、负向文本、占位符和缓存正文。整图参考作为latent时按原图片生成的冻结身体/衣柜版本核对，不能靠最新聚合元数据宣称旧图片与新穿搭一致。

没有证实聊天完整Core Persona直接注入媒体的路径，也没有读取运行数据库证明用户实际workflow包含固定身份节点；这两项不作为已确认根因。

## 输入、人物与配置行为
Tool保留intent/capture，新增可选media_plan、visible_subjects、human_count、referent_bindings、reference_asset。self别名指任务Actor；第一人称指代可区分task_actor/actor_user/unknown，不据作者或所有者自动加载人物。已知其他Fluctlight ID须通过Owner权限检查，各自独立投影当前资料；匿名人物仍由原意表达。Human/未知ID没有Fluctlight视觉资料，不能借此虚构身份。

MediaCapturePlan采用八个枚举字段：primary_subject/human_presence/self_in_frame/visible_scope/capture_method/viewpoint/framing/camera_angle。原intent原文不被Kev覆盖，摄影字段可unspecified。Field sources记录explicit/kev/original_llm/neutral_default，准备重放校验全部输入digest。

关闭Kev后仍使用分支解析与中文规范化。主体无法确定时不猜自拍；异常仅通过同一媒体模型基础层的有界解析，取消不另起回退。完整可靠声明可零Kev调用。普通路径通常两阶段；未知主体/身份只允许一次原模型解析。已耗尽的一次规范化修复返回不可重试media_prompt_validation_failed，实际Worker将其转换为Temporal非重试错误并立即记录终态，后台Activity重试不重启预算。网络错误保留原重试策略。

统一media:<intent ID>关联准备、Kev、规范化、提交，准备事件保留parent_correlation_id；实际请求/响应继续在原Owner诊断中查看。

Kev问题版本：kev.media.subject_route.v1 / kev.media.capture_plan.v1 / kev.media.view_plan.v1，状态表示media.capture.v1。候选见mediaPlanOptions，unknown弃权，unspecified为中性采用值，不新增统一0.8门槛。默认配置的模型标识仍Jakevin/kev-4b-ternary-mlx；model_revision未知或沿用已有值。本轮未接真实服务，未确认权重哈希/runtime版本，不把kev-latest作为版本。

## 五类契约轨迹样例
下表来自固定契约fixture与其装配/校验断言，**不是真实模型/图片运行记录**。固定stub正文为原意原文；真实中文规范化效果由用户执行C层检查。

| 场景 | 计划要点 | 允许资料 | 排除与最终条件 |
| --- | --- | --- | --- |
| M01 无人雨后街道 | landscape/none/no；拍法unspecified | 原意、约束、视图 | 本人外观查询0、身份查询0；无人物参考/身体LoRA |
| M09 手持咖啡局部 | person/partial/yes/hands | 手部相关肤色/手套及环境快照 | 无脸、头发、身高、胸杯和完整身份图 |
| M11 平视前置近景 | person/present/yes/face；handheld_selfie/eye_level | 稳定脸部身份、当前可见头发与上身信息 | 无鞋/身体LoRA参数，不补高机位 |
| M12 对镜全身穿搭 | person/present/yes/full；mirror_selfie/reflection/full_body | 稳定身份、当前身体与当前穿着 | 保留镜面和设备关系；动态身体权重来自当前杯型 |
| M14 公园他拍全身 | person/present/yes/full；external_photo/full_body | 本人当前资料和原意中的场景 | 不生成摄影师Actor，不套手持自拍，保留头顶到脚底要求 |

请求形状以M01为例：
```json
{"intent":"窗外雨后的街道，路面反光，没有人。","media_capture_plan":{"primary_subject":"landscape","human_presence":"none","self_in_frame":"no","visible_scope":"unknown","capture_method":"unspecified","viewpoint":"unspecified","framing":"unspecified","camera_angle":"unspecified"},"allowed_subject_ids":["scene","animal","object"]}
```
实际内部还可有explicit_constraints、referent_bindings、declared_capture；none不会有allowed_subject_data中的本人信息。本人/局部资料在allowed_subject_data中按self分区；已知他人在other_subjects中按subject_id分区。仅允许引用的资产进入allowed_reference_assets。最终请求为现有`POST /prompt {prompt: <已替换且已检查的workflow>}`，不是把声明字段发给Comfy。

## 测试产物与执行方式
固定中文场景数据：apps/core-go/internal/core/testdata/media_capture_cases.json，M01–M29各两种表述；M30–M40特殊情况在TestMediaBranch*中覆盖顺序/并发、混合主体、冲突/原答案、关闭/错误、当前外观、修复、参考条件与取消/撤销。还有真实Worker非重试边界及0060迁移保留/重跑用例。含污染sentinel的角色/服装/身体/参考资料用于检验真实装配函数边界。

A层：自动化工程用例已运行；最终数量/命令结果见下方验证记录。
B层：真实Kev单点未执行。
C层：真实规范化模型未执行。
D层：实际生成与人工图片检查未执行。
数据库、Temporal部署恢复和生产浏览器交互亦未作实测；SKIP不能计为PASS。两次独立只读代码核验的已确认问题由主代理修改；最终窄范围复核没有新的P1问题。

从仓库根目录运行工程用例：
```sh
go -C apps/core-go test -race ./...
go -C apps/core-go vet ./...
pnpm generate
pnpm typecheck
pnpm test
pnpm build
```

B独立入口，只访问指定Kev，不访问规范化/Comfy或发布：
```sh
FLUCTLIGHT_MEDIA_KEV_URL='<实际SystemOne URL>' FLUCTLIGHT_MEDIA_REPORT=/tmp/media-kev-report.json go -C apps/core-go test ./internal/core -run '^TestMediaPlanProbe$' -count=1 -v
```
可选FLUCTLIGHT_MEDIA_KEV_API_KEY从环境读取，不打印。JSON报告保留期望/实际方案、错误、原始逐题记录、分布和延迟；允许值需按用例业务范围审阅，不以未指定角度与expected完全相同冒充正确率。

C独立入口，使用已有真实Provider测试参数与可销毁数据库：
```sh
FLUCTLIGHT_MEDIA_PROBE=1 FLUCTLIGHT_MEDIA_MODEL_MODE=live FLUCTLIGHT_MEDIA_CASE=M01 go -C apps/core-go test ./internal/core -run '^TestMediaNormalizationProbe$' -count=1 -v
```
同时配置GO_CORE_TEST_DATABASE_URL、FLUCTLIGHT_LIVE_PROVIDER_TEST=1、FLUCTLIGHT_LIVE_PROVIDER_URL、FLUCTLIGHT_LIVE_PROVIDER_MODEL；凭据用FLUCTLIGHT_LIVE_PROVIDER_API_KEY。真实fixture创建并清理隔离测试库，不写生产库。FLUCTLIGHT_MEDIA_STATE_FILE可提供可控授权快照；FLUCTLIGHT_MEDIA_INTENT覆盖原意并由真实媒体解析处理。报告默认/tmp/fluctlight-media-probe/<case>-live.json，含实际模型请求及最终工作流。工程stub模式可选，但按用户要求本轮没有单独执行该探针。

D在C参数上显式增加FLUCTLIGHT_MEDIA_COMFY_EXECUTE=1、FLUCTLIGHT_MEDIA_COMFY_CONFIG_FILE=<私有兼容配置文件>，可用FLUCTLIGHT_MEDIA_REPORT_DIR指定输出目录。建议逐个M01/M09/M11/M14执行；不默认批量生成，不真实发布。独立图片探针不读取私有身份资产，配置中不能含需要私有身份上传的占位符；需要完整身份引用/Worker/S3验收时使用已有生产Worker隔离fixture。真实图片文件/任务ID写报告，仍需用户人工核对；文本正确不等于像素正确。

## 用户测试前的配置与迁移
1. 在目标本地环境以原部署参数执行`go -C apps/core-go run ./cmd/migrate`，到达0060_media_kev。不要对未授权远程环境执行。
2. 设置UI已有媒体总开关及三个分项。可用`go -C apps/core-go run ./cmd/kev-config --media-only --backup /tmp/kev-before-media.json`合并启用；首次配置另传--endpoint。备份须新路径，0600、fsync，失败不更新。原已关闭全局时旧七点保持有效关闭；只禁用媒体时加--disable保留已有全局/七点效果。
3. media.comfyui可配置media_workflows.{none,unknown,partial,other,self,face,back}；否则复用原workflow并检查。只有人物参考的旧workflow必须提供相应兼容分支；缺可用分支时明确报错，不发送已知污染条件。
4. style_loras列合法风格LoRA完整文件名；face_identity_loras列脸部身份LoRA。身体LoRA用当前{{chest_lora_weight}}占位符，不能隐含固定旧身体值。quality_negative_prompts列经确认只用于质量排除的完整负向字符串，未分类负向不静默携带。
5. 普通场景/动物/物品引用使用{{media_reference_image}}与对应reference_asset用途；本人身份引用使用{{visual_identity_reference_image}}。不把两通道混用。无法验证来源的历史资产会报错，需要可验证资产或兼容配置。

## 回退与限制
代码旧默认备份：research/legacy-defaults.md；实际设置备份由上述操作命令在用户环境生成。关闭媒体Kev回到修复后的原模型路径，隔离修复仍有效。恢复配置只恢复备份中的相关kev/media.comfyui值，不重置衣柜、地点、目标或已生成资产；旧备份缺media_enabled时应显式关闭新增媒体增强。

0060是新增ledger版本，直接切回仅识别0059的旧二进制会被迁移版本检查拒绝；代码回退需配套处理schema兼容，不能删除审计或业务事实来降级。本轮未执行数据库升级或回退。

确定性规则覆盖已知模板骨架、结构、主体/参考ID和图条件；任意自由文本、复杂自定义Comfy节点内部行为、模型偏好及构图仍需要用户真实抽查。普通匿名人物依据原意，已知Fluctlight可按授权ID读取；未新建Human视觉身份查询/训练体系。无兼容工作流/未分类条件时宁可返回可诊断配置错误，不能宣称图片已经正确。

## 最终验证记录

| 检查 | 结果 |
| --- | --- |
| go -C apps/core-go test -race -json ./... | 退出0；1549 PASS / 510 SKIP / 0 FAIL（含父测试与子测试事件） |
| go -C apps/core-go vet ./... | 退出0 |
| pnpm generate | 退出0，未产生额外契约漂移 |
| pnpm typecheck | 退出0，三个工作区包通过 |
| pnpm test | 退出0；Web 76、browser-client 20、core-client 2，合计98项通过 |
| pnpm build | 退出0；保留已有Vite >500 kB chunk提示，无构建错误 |
| git diff --check / Trellis context validate | 退出0；两个manifest各10条，未截断 |

SKIP包含未配置隔离数据库/真实Provider的集成与验收，以及本轮不执行的独立探针。详细分包计数见research/validation-results.json。工作区客户端测试首次因沙箱tsx IPC监听EPERM中断，在沙箱外重跑通过；此环境失败未计作代码缺陷。

补充：规范化JSON输出现已包含declared_human_count，明确人数在结构层锁定；referent_bindings支持task_actor/actor_user/unknown。半身请求的模型资料排除鞋履与无关身高。整图身份latent使用不可变来源快照核对当前身体/衣柜版本，固定不兼容参考返回media_reference_current_state_conflict。
