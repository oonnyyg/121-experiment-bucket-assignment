# experiment-bucket-assignment

离线实验分桶工具（仅 Go 标准库）。将同一批用户稳定分配到互斥实验及其版本，
输出逐用户 JSONL 决策与 CSV 分布汇总，不连接任何线上服务。

## 用法

```
go run . -config config.json -users users.csv -decisions decisions.jsonl -summary summary.csv
```

- `-config`：实验配置 JSON（必填）
- `-users`：用户 CSV，首列必须为 `user_id`，其余列为用户属性（必填）
- `-decisions`：决策 JSONL 输出路径（默认 `decisions.jsonl`）
- `-summary`：汇总 CSV 输出路径（默认 `summary.csv`）

配置或用户文件有误时直接报错退出，不产生任何部分分配结果。

## 配置格式

```json
{
  "groups": [
    {
      "name": "checkout",
      "salt": "group-salt-1",
      "experiments": [
        {
          "name": "new-flow",
          "salt": "exp-salt-1",
          "start": 0,
          "end": 5000,
          "attribute": {"key": "country", "values": ["US", "CA"]},
          "variants": [
            {"name": "control", "weight": 5000},
            {"name": "treatment", "weight": 5000}
          ]
        }
      ]
    }
  ]
}
```

规则：

- 组名与实验名各自全局唯一；同组实验桶段为 `[start, end)`（0–10000），不得重叠；
  未覆盖的桶不进入任何实验（原因 `bucket_not_covered`）。
- `attribute` 可选：要求用户属性属于指定字符串集合；缺失属性视为不满足，
  命中桶但不满足条件时用户保持未分配（原因 `attribute_mismatch`），不顺延到同组其他实验。
- 版本权重为正整数且总和恰为 10000，按声明顺序累计成桶段。

## 分桶算法

SHA-256 输入为 `标签、盐、对象名、用户标识` 依次以空字节 `\0` 连接的 UTF-8 字节串
（所有组成部分禁止含空字节）。取摘要前 8 字节按大端无符号整数解释，对 10000 取余。

- 组选实验：标签 `group`，对象为组名，使用组的盐。
- 实验选版本：标签 `variant`，对象为实验名，使用实验的盐。

桶号只取决于上述输入，与配置中实验声明顺序、用户文件顺序无关。

## 输出

**decisions.jsonl**：每行一个用户：

```json
{"user_id":"u1","decisions":[{"group":"checkout","group_bucket":1234,"experiment":"new-flow","variant":"control","variant_bucket":42}]}
```

未分配时省略 `experiment/variant/variant_bucket`，并给出 `reason`
（`bucket_not_covered` 或 `attribute_mismatch`）。

**summary.csv**：`group,experiment,variant,reason,count`，计数由同一份决策明细
累计得到，保证与 JSONL 明细一致。同组每人最多进入一个实验，不同组相互独立。

## 测试

```
go test ./...
```

覆盖：桶边界（0/4999/5000/7999/8000）、属性门槛与缺失属性、版本权重分布、
组间独立性、用户顺序无关性、配置错误（重名、重叠、越界、权重和不为 10000、
空字节）及配置错误时不产生输出文件。
