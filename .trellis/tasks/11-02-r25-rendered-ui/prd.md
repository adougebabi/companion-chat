# R25 实际渲染 UI 修复

在最新提交 09a1b46f 上独立修复 P1-4、P2-1、P2-2、P2-3、P2-4、P2-5、P2-8。
以 /workspace/ux/demo/fluctlight-demo.html 和 R3 截图为基准，逐项追踪生产 DOM，新增回归，生产构建 + 无头 Chrome 验证。
不提交 Git，不读写 *.env 或 /workspace/accept，不修改真实服务数据，不削弱旧断言；截图仅放 /tmp/r25-shots。
