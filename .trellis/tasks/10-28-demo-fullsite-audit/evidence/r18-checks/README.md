# r18 checks

web-tests.log/typecheck.log/build.log为最终检查。modules.log保留首轮无正文场景失败（目录拦截漏详情），由unwritten.log与r18-unwritten3报告替代；该失败未计入88条通过索引。priority、wizard-stepper、task-focus分别覆盖对应增量，不能相加成页面数。所有浏览器请求使用隔离HTTP；core-workflow是纯Go元数据测试，不是实时Core验收。
