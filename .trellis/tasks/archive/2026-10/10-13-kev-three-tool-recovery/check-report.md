# 本地验证与独立核验

2026-10-13，主代理实施与最终验证，子代理只读定位/核验。没有真实模型、真实数据库或生产回归。

- Go全包测试、vet、build通过；Core/decision race通过（结果在提交前核对）。Release Python用例3项通过，git diff --check通过。
- 公共实际Generate/Stream初始HTTP请求精确catalog/reply/moment；named catalog audited yes动态加载，fresh stage、拒绝/不确定/关闭/审计失败、替换窗口最大11、foreign请求原子拒绝通过。
- Writer原生图片请求的实际两轮HTTP提供单一reply.image.request，真实receipt返回requested_not_generated；正文空行/字面\n归一化与fence/inline code保留通过。新JSON媒体请求无native receipt拒绝，placeholder不会冒充附件。
- 纠错规则独立于CurrentInput，二次HTTP当前用户文本一次、两完整最近轮保留；一次纠错有界。
- 中文合成200条长history：31708候选历史字符→1296已选历史字符。最终含私有图片Tool schema wire字符4205，保守估算5219 input tokens。英文105108→1079历史字符，final wire3799字符/4712估算tokens。保留persona/current refusal/original Goal条件，summary覆盖去重、原projection不变。不声称实测性能或真实模型tokens降幅。
- PG用例ReplyEvidenceRecoveryBeforeWriterLease/ReplyMediaCatalogDenialCompletesSameDraft已编写，未设测试DB的本地运行明确skip；加入CI一次性PG门禁，需远程受控结果确认后再记录发布完成。
- 两位只读核验：Kev无已确认P1/P2；Writer指出fresh JSON媒体请求绕过入口及纠错current重复，两项已修复并补定向测试。普通单literal换行与inline/fence共存边界已修正。公共dedicated bypass显式清除继承selection。

## 最终CI与发布确认

b024f35c7d0d585f748817bd0297366c14a6d194 / run 38423006085 全部成功。完整Go测试/race、Phase8 PG gate、fact-only定向PG/race、vet/build/gofmt、Web生成/类型/测试/构建均通过。引用纠错与真实失败terminal两个DB分支通过；目录denial→同draft纯文本发布、媒体任务0通过。首轮PG运行失败于最后统计断言错误列fluctlight_id，已按真实schema改为owner_fluctlight_id，复跑通过。Core/Web测试镜像及完整published清单已核对远程digest/源标签。发布证据位于runs/publication（本地忽略）。实现及测试提交均已推送功能分支，归档/日记由Trellis在本地记录。真实模型性能/图片表现由用户真实环境验收。
