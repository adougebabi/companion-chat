# 旧 .trellis 归档与恢复

旧目录已移出工作树，AEL仍为唯一活动工程流程。391条source字段只是原始身份，保留不改。

完整归档：`runtime:archives/trellis-retirement-20261103/trellis.tar.gz`，用`ael doctor --project .`定位运行目录。SHA-256：`79af69b092f0ddaeb4102eb847cdfb2483798b54727910bfdd47f9820b5d9a52`。共1248文件、57825995字节。

## Archived originals

历史实验源码以只读.txt快照链接保留，未执行；其中旧模型参数不代表现行规则。原始日志、脚本、二进制均在完整归档中。

## Missing originals

以下原引用路径在本次清理前就已不存在，未编造原件：
- `.trellis/spec/backend/error-handling.md`
- `.trellis/spec/guides/cross-layer-thinking-guide.md`
- `.trellis/tasks/10-15-fluctlight-functional-recovery/evidence/full-stack/concentrated-nonllm/about-you-desktop-final.jpg`
- `.trellis/tasks/10-15-fluctlight-functional-recovery/evidence/full-stack/concentrated-nonllm/about-you-mobile-final.jpg`
- `.trellis/tasks/10-15-fluctlight-functional-recovery/evidence/full-stack/concentrated-nonllm/npc-editor-desktop.jpg`
- `.trellis/tasks/10-15-fluctlight-functional-recovery/evidence/full-stack/execution-1016.json`

## 恢复

回滚旧迁移前，先核对归档SHA，解包到临时目录，再将完整.trellis恢复到项目根；已有同名目录时停止并核对，不能覆盖新内容。然后运行原ael migrate rollback命令。旧控制器不会因归档恢复自动开启。恢复脚本和完整文件清单与归档相邻（restore.sh、files.json），可用`ael doctor`给出的目录定位。

本次只改了历史链接和验收日志位置，退休了已无调用方的旧handoff检查脚本。真实模型/数据库测试没有执行，看板没有启动。
