# Design

## Boundaries

完整 `core_persona`、foundation revision 和有效稳定覆盖层是来源。编译 Agent 通过现有 `RunFormalAgent` 调用模型，只输出结构化画像和诊断，不执行 Tool 或状态修改。现有 Working Persona 作为唯一正式运行时人格载体，保存编译产物后确定性渲染。动态状态沿现有 ContextProjection 与 Prompt Composer 装配。

## Lifecycle

初始化在短事务外编译所有声明 profile，验证后与来源版本一起发布。正式 foundation 或稳定覆盖层更新时按受影响 profile 从完整来源重新编译，版本 CAS 防止过期结果覆盖。运行时校验来源与产物版本；缺失或失配报错。补编译命令按实例预览、跳过、重试，不改动态状态。

## Query contract

同一只读 Capability 提供 list/read；执行上下文绑定 Fluctlight、实际发言 profile 和权限，模型参数只选择 section 与继续位置。section 来自完整结构化人格确定性分区，read 按完整事实边界有界返回。独立 `App.ExecuteTool` 与 Eino 原生循环共享业务实现，结果保持数据身份。

## Compatibility and rollback

保留权威完整资料及治理接口。准备存量产物后切换正式 Prompt，不能永久回退完整人格注入。更新失败保留上一个一致有效版本并返回诊断；数据库变更使用项目现有 migration 机制。真实 Provider 凭据不可用时仅标记验收阻塞。
