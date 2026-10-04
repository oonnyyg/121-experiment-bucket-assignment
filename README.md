# experiment-bucket-assignment

离线实验分桶工具：读取 JSON 实验配置与用户 CSV，输出逐用户 JSONL 决策与 CSV 分布汇总。仅使用 Go 标准库，不连接任何线上服务。

## 配置

见 `examples/config.json`：

- `groups[]`：互斥实验组。`name` 全局唯一，`salt` 为组级盐。
- `experiments[]`：组内实验。`name` 全局唯一；`bucket_from`/`bucket_to` 为 [0,10000) 内的左闭右开整数桶段，同组不得重叠，未覆盖桶不进入任何实验。
- `attribute` / `allowed`（可选）：要求用户该属性值属于指定字符串集合；属性缺失视为不满足。命中桶但不满足条件的用户保持未分配，不顺延到同组其他实验。
- `variants[]`：版本。`weight` 为正整数且总和必须为 10000，按声明顺序累计成桶段。

所有名称、盐、属性名禁止包含空字节；配置校验失败时不产生任何输出。

## 分桶算法

SHA-256 输入为 `标签、盐、对象名、用户标识` 依次以空字节（`\0`）连接的 UTF-8 字节。取摘要前 8 字节按大端无符号整数解释，对 10000 取余：

- 组选实验：标签 `group`，对象为组名，使用组盐。
- 实验选版本：标签 `variant`，对象为实验名，使用实验盐。

桶号只取决于上述输入，与配置中声明顺序、用户顺序无关；同组每人最多进入一个实验，不同组相互独立。

## 输入

用户 CSV 第一列必须为 `user_id`（唯一、非空、不含空字节），其余列为属性：

```csv
user_id,country
user0001,US
```

## 输出

决策 JSONL（每用户每组一行）：

```json
{"user_id":"user0001","group":"homepage","group_bucket":4093,"experiment":"layout","variant_bucket":245,"variant":"control"}
{"user_id":"user0002","group":"homepage","group_bucket":8300,"reason":"no_experiment"}
```

未分配原因：`no_experiment`（桶未覆盖）、`attribute_missing`、`attribute_not_allowed`。

汇总 CSV 列为 `group,experiment,variant,reason,count`，计数总和与决策明细行数一致（程序内部校验）。

## 使用

```sh
go run . -config examples/config.json -users examples/users.csv \
  -decisions decisions.jsonl -summary summary.csv
```

## 测试

```sh
go test ./...
```

覆盖：已知哈希向量、桶范围与空字节拒绝、相邻/重叠桶段校验、权重和校验、属性门槛（缺失/拒绝/通过且不顺延）、版本权重分布、端到端明细与汇总计数一致、重复用户拒绝。
