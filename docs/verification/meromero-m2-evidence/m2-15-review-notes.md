# M2-15 动态关系证据投影

隔离 PostgreSQL 中创建关系后，本项使用真实 Core 引用索引和 Moment 准备入口，
将已选关系交给受控 Writer。起始提交 `50d3b0be42d53fcbddba857bae3338c9bf46ec07`
的物理请求丢失关系业务对象中的 `id`、`source_ref` 和 `revision`，红测见
`m2-15-red.log`。原数据是测试夹具；未向真实用户发布动态。

修复将关系的已声明业务字段与 Core 外壳分开，并将已解析的对象映射为语义人物；
未解析的人物保留 `unknown`。最终候选的 2 项定向测试、11 项相关回归和含四处
原有草稿的工作区 2 项测试均退出 0、无 SKIP，见 `m2-15-final.log`、
`m2-15-regression.log` 和 `m2-15-working-tree.log`；受检哈希见 manifest。

生产写入路径核对未发现 Core 自动把内部引用写入 `emotional_association`；该
开放对象仍按业务数据原样保留，因此输入或历史数据本身如果包含类似内部标识的
字符串，也不会仅按字段名删除。通用 JSON 大整数精度已有独立工单，本项没有
声称解决。受控 Provider 不代表真实模型效果，其他 Moment 证据类型和 T03/M2
整体验收仍未完成。
