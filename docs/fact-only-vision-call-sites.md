# 实际视觉入口

| 入口 | 实际图片来源 | gate / binding | 关闭或未配置行为 |
| --- | --- | --- | --- |
| RunMediaQualityTask / evaluateMediaQuality | ComfyUI 下载后的真实候选字节，含视觉身份/设定卡 | resolveVisionCapability → vision_understanding → Provider/physical guard | skipped，继续对象保存与生成结算，不冒充 pass |
| RunVisualIdentityAgent commit_review | 当前 candidate ready 资产的对象字节 | review 前 resolver；可信冻结 vision assignment；native session/asset/config provenance | awaiting_review，保留 candidate 与旧 canonical，停止自动 polling |
| VisualIdentity generate_candidate / finalize | 文字状态与真实 ready 检查，无图片输入 | 原 cognitive text binding | 不由视觉开关禁止 |
| RunVisualIdentityVisionTask | 调用者实际 image content | resolver 与独立 vision formal role | 明确 vision_disabled / unconfigured，不猜图 |
| direct visual_identity.commit_review | 直接参数本身不是图像 | native 私有 inspection authority，或明确 Owner manual provenance | 没有来源拒绝；人工不标记模型已观察 |
| 原始 Provider completion / StreamText | 所有支持的 image/image_url block | assignment/secret 前 gate，Generate/Stream physical guard；ADK 排队实际提交再检查 | 零图像 HTTP；配置读取失败也拒绝 |
| Owner 显式 vision/test | 随机两色 PNG | 已保存 independent vision binding，Owner/CSRF | 关闭不请求；测试不自动保存配置或加载模型 |

用户聊天附件目前只有 refs，未新增把上传附件读取为像素并请求理解的流水线；收取/显示附件不代表模型已看图。头像为现有首字母显示。当前视频链没有抽帧理解任务，生成配置保留；物理 image guard 也覆盖未来支持的图片调用者。media_assets ready 继续表示字节可读，检验状态存在质量 verdict / visual attempt provenance 中。
