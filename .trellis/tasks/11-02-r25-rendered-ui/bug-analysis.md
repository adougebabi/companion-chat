# Bug Analysis: R25 审计与实际渲染路径

## 1. 根因类别
C（改动传播遗漏）：上一轮创建只翻译页头，未翻译继续后的共享侧栏与准备区 eyebrow。
D（覆盖不足）：导入只测独立 FilePicker 的 wrapper，未统计真实 /data 中 outline Button 的额外边框。
E（状态假设）：不能由 R3 PNG 断言当前分支改错组件；必须比对当前 production dist。

## 2. 上轮为何没有体现
- P1-4、P2-2、P2-3、P2-4：09a1b46f 修改前生产构建在相同路由和空状态已通过；当前 13001 GET 静态资源也包含对应修复。R3 图片与当前产物不同，无法仅凭 PNG 判定当时未部署、旧 HTML/chunk 或标签页缓存中的哪一种原因。没有复现另一条生产渲染入口。
- P2-1：实际已是 Field + FieldLabel + Switch，只有 CSS 仍留 1px 底边，本轮全部移除。
- P2-5：实际页头已为中文，共享侧栏仍是 YOUR NEW CHAPTER，准备区仍是 PREPARING A LIFE；覆盖范围遗漏确证。
- P2-8：实际 FieldPort 已传 framed=false，但 FilePicker 的 Button 仍固定 outline；wrapper 的旧断言通过并不代表整块导入区只有一层框。

## 3. 防复发
P0：真实创建页面从 world 继续到 source；真实 data-tools → DomainPrimitives → FilePicker 的文件传递（已补）。
P0：生产 dist 的七项 DOM/几何断言在桌面和窄屏执行，保存 baseline 与 after JSON（已补）。
P1：保留 framed=true 的 outline 与 disabled drop 行为测试，避免扩大共享组件影响（已补）。

## 4. 范围
没有理由修改路由、缓存部署设置、服务端、提示词或业务协议。当前已正确的四项只加强覆盖。

## 5. 知识沉淀
具体真实入口、框线所有权和 Radix 测试注意事项已写入 frontend/quality-guidelines.md。
