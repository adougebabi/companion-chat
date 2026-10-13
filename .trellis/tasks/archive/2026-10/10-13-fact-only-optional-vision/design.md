# FO-E 可选视觉理解全入口治理 技术设计

## 整体设计依据

依父任务 ../10-13-fluctlight-fact-only/design.md 执行；共同跨子任务合同不可被局部替换。

## 本子任务技术边界

统一gate在domain入口/执行前和physical image-bearing请求边界检查，新增inspection provenance但不改media_assets ready含义。media quality关闭复用skipped→真实保存/发布；运行失败按optional规则分类。VisualIdentity非图片generate/finalize用原文字能力，只有commit_review图片阶段用vision；disabled/unconfigured停awaiting_review保存未检验candidate，canonical/CAS不放宽。覆盖RunMediaQualityTask、RunVisualIdentityAgent review、旧VisionTask、direct commit_review与queued任务。direct review须服务端视觉来源，人工确认只能manual provenance。当前用户附图仅refs、无像素读取入口，保持接收并表示未理解；不新增上传理解平台。catalog/discover与WakeUp缺视觉条款同步，重启规则用现settings读取，重新enabled不补跑旧skip。

## 兼容与失败

T12/13：disabled/unconfigured无请求、真实图片只发指定Provider、伪视觉/超时不pass、后台无热重试。T14/15：生图/视频开关不受影响、bytes-ready与inspection分离、初始化继续且awaiting_review不反复补跑。T19在途workflow/状态兼容。

## 证据

- ../10-13-fluctlight-fact-only/research/vision-media.md
- ../10-13-fluctlight-fact-only/research/settings-context.md


## 用户最新验收分工（2026-10-13）

用户已确认开始实施，并补充“本地测试用例就行，不会回归，我真实环境回归”。本次代理负责本地新增/受影响测试、类型检查与构建；不执行真实模型长聊、真实环境回归或部署/回滚演练。提供用户可执行的配置与验收步骤，报告明确标记这些验证由用户执行。本条覆盖之前要求代理执行完整回归/真实模型/环境演练的执行安排；功能范围、权限、事实/发送边界与可回滚设计保留。无需再次请求规划确认。
