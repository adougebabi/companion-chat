# 前端单元测试

`apps/web/test/*.test.mjs` 用 node:test 运行，React 组件在 jsdom 中渲染。浏览器里的布局、
交互与视觉检查见 [前端 UI 质量门禁](frontend-ui-quality.md)，单元测试不能代替它。

```bash
pnpm --filter @fluctlight/web test           # 全量：验收以它为准
pnpm --filter @fluctlight/web test:changed   # 日常：只跑受未提交改动影响的测试
pnpm --filter @fluctlight/web test:changed -- --base master   # 相对某个提交
pnpm --filter @fluctlight/web test:changed -- --list          # 只列出会运行的测试
```

## 源码如何加载

测试不再为每个文件启动 Vite 开发服务器。`node --import ./test/support/register.mjs`
在每个测试进程里装上同步模块钩子（`test/support/module-hooks.mjs`）：

- `.ts/.tsx/.jsx` 以及读取 `import.meta.env` 的 `.js/.mjs` 用 esbuild 编译，这个 esbuild
  就是 Vite 自己依赖的那一份。`import.meta.env` 的值和 Vite 开发服务器一致：
  `MODE=development`、`DEV`、`SSR`，以及环境变量里的 `VITE_*`。
- 支持 `@/` 别名，以及 Vite 允许的省略扩展名、用 `.js` 指向 `.ts` 的写法；CSS 导入为空模块。
- `node_modules` 里的包由 node 原生加载，与 Vite SSR 的 external 处理一致。
- 测试里的 `createServer` 来自 `test/support/module-server.mjs`，只提供测试实际用到的
  `ssrLoadModule(url)` 和 `close()`。和多个 Vite 服务器一样，每个 server 拥有自己的一套
  项目模块实例：URL 上的 `?vite-instance=N` 会沿项目内的导入链传递。`node_modules`
  里的包在各 server 之间共享。
- 需要真实生产构建的测试（`inbox-toolbar-cascade`，检查构建后的 CSS）仍然调用 Vite `build`。

## 并发

`scripts/unit-test.mjs` 同时运行「可用 CPU 数 + 1」个测试文件，可用 `UNIT_TEST_CONCURRENCY`
覆盖。node 默认只开「可用 CPU 数 − 1」个，在受 cgroup 限制为 2 核的机器上就只剩 1 个，
测试文件实际上是逐个跑完的。测试进程有一部分时间在等文件 I/O，所以多开一个能把 CPU 用满。
2 核上的 40 个文件实测：并发 1 为 127 秒，2 为 78 秒，3 为 71 秒。

## 缓存

缓存放在本机磁盘 `~/.cache/fluctlight-web-unit-tests/<检出目录哈希>/`（或
`$XDG_CACHE_HOME`），不放在检出目录里，因为检出目录可能挂在网络文件系统上。

| 内容 | 失效条件 |
| --- | --- |
| `modules/`：编译后的源码 | 内容哈希；源码、esbuild 版本、编译选项或 env 变化即不命中 |
| `resolve-<安装键>.json`：包的模块解析结果 | 安装键 = `pnpm-lock.yaml` 内容 + `node_modules/.modules.yaml` 修改时间 + node 版本 |
| `packages-<安装键>/`：包文件的本地副本 | 同上 |
| `deps/`：每个测试文件实际加载和读取的项目文件 | 该测试文件下次通过时覆盖 |

重新安装依赖会换一个安装键，旧目录可以直接删除。遇到怀疑与缓存有关的问题，删掉整个
缓存目录即可恢复，下一次运行会重新建立。

实测（检出目录挂在 sshfs 上，cgroup 限制 2 核，1375 项全部通过）：原方式约 62 分钟。
原方式下每个测试文件都要冷启动 Vite，并经网络逐个查找、读取约 2000 个模块文件，
而且受默认并发所限，文件是逐个运行的。改用钩子和缓存后约 14 分钟，再调整并发后约 7.8 分钟。

## 只跑受影响的测试

`scripts/unit-scope.mjs` 把改动文件（相对 `--base`，默认是 `HEAD` 的未提交改动，加上未跟踪文件）
与 `deps/` 里的记录对照，以下几类测试文件会运行：

- 测试文件本身改了；
- 它记录的任一项目文件改了，或它读取过的目录下有文件增删；
- 它还没有记录：新文件，或上次没有通过。

以下改动会触发全量运行：`test/support/`、`test/helpers/`、依赖清单与锁文件、`tsconfig.json`、
范围脚本本身。没有被任何测试读取的源码改动会单独列出来。这只是日常的快速反馈，结论里也会注明；
验收和提交前仍以全量 `test` 为准。
