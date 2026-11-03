# A 独立核验
2026-10-07 a_correctness_review 返回五项：失败结算锁序、direct未来timer、harddeadline永久due、未知operation无限续租、标准ID文本派生。全部纳入修复：共享Life先锁、typedtime成熟度、硬期限expire、读取真实operation并六次未知后停止自动核对、criterion_ids持久化与历史snapshot，提交使用ID+版本。
原始ee1e468复跑14个广泛回归失败均同样失败，记录父 research/baseline-failure-comparison.jsonl；不当新回归或PASS。数据库测试基础postgres先执行migrate，孤立helper只创建随机库。
A-specific tests日志在父research/a-*.jsonl；接下来的B负责完整证据Evaluation/逐标准状态与来源失效，C负责Review/API/UI，D负责完整矩阵/真实provider/媒体联合验收。
