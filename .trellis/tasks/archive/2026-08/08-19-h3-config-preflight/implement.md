# Implementation Plan

1. [x] 增加 h3 路径/权限/目录检查与安全 display summary helper。
2. [x] 在 h3 设置保存路径增加有条件的 server-side validation；扩展 public settings 的安全摘要。
3. [x] 实现 debug-only h3 `--help` preflight，短超时、`shell:false`、受限输出，无 queue 副作用。
4. [x] 在设置页回显摘要，在开发检查器添加预检入口/结果渲染。
5. [x] 添加配置保存、预检无副作用、debug route gate、输出脱敏测试。
6. [x] 运行 `npm test`、`node --check server.js`、`node --check src/companion-main.js` 与 `git diff --check`。
