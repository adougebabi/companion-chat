# Technical Design: h3 配置回显与运行预检

## Settings Contract

保留当前 `companion_settings` JSON 存储。新增派生字段 `h3ConfigSummary` 到 `publicSettings()`，其中每项只返回是否配置、`…/basename` 展示名和合法性，不返回完整绝对路径。

## Validation

`validateH3Configuration(config, {ensureOutput})` 统一检查：`h3Executable` 必须是绝对路径、普通文件并具有执行权限；`h3ModelDir` 必须是目录；`h3OutputDir` 必须位于 `h3AllowedRoot || h3OutputDir`，可创建且可写。保存 h3 字段或选择 `videoProvider: 'h3'` 时执行该检查，失败不写 SQLite。

## Debug-only Preflight

在现有 debug gate 内增加 `POST /api/companion/h3-preflight`。它使用当前运行服务的 settings 运行文件系统检查，再以 8 秒超时、`shell:false` 执行 `h3 --help`。返回脱敏、截断后的摘要；不读 prompt、不创建 job/资产、不执行生成。

## UI

设置页 h3 区展示“当前服务配置”安全摘要。开发检查器增加“测试当前 h3 配置”按钮，显示检查中、成功或失败信息。浏览器只调用本应用 API，不直接启动 h3。

## Rollback

移除预检 route/按钮不会影响正常 h3 job；安全摘要为派生字段，不需要迁移历史数据库。
