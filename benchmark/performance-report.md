# Mory 性能数据

这份数据用于展示 Mory 作为 Markdown 编辑器的性能表现。所有数值为 3 次独立运行的 **中位数 / P95**，单位为毫秒或 MiB。

![Mory 性能概览](performance-overview.svg)

## 测试负载

测试使用仓库中的 6 个 1 MiB Markdown fixture，在 Apple M4、macOS 27.0.1、24 GiB 内存和 2560×1312 窗口下运行。

| Fixture | 大小 | 行数 | 最大行长 |
| --- | ---: | ---: | ---: |
| `long-line-1m` | 1.00 MiB | 1 | 1,048,576 |
| `memory-heavy-1m` | 1.00 MiB | 39,147 | 123 |
| `structured-1m` | 1.00 MiB | 60,964 | 46 |
| `table-heavy-1m` | 1.00 MiB | 2,404 | 1,025 |
| `table-long-cells-1m` | 1.00 MiB | 402 | 3,589 |
| `target-shape-12x1m` | 1.00 MiB | 3,207 | 769 |

## 核心性能数据

| Fixture | 模式 | 打开 action | 加载后 RSS | RSS 峰值 | 编辑 action | 保存 action | Source 滚动 capture |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `long-line-1m` | Write → Source | 64.9/74.1 | 197.7/199.5 MiB | 249.0/253.2 MiB | 0.0/0.0 | 31.2/32.0 | — |
| `long-line-1m` | Source | 59.2/60.7 | 198.7/198.8 MiB | 254.8/254.9 MiB | 0.0/0.0 | 34.4/35.8 | 54.9/65.3 |
| `memory-heavy-1m` | Write | 222.9/223.3 | 409.1/410.2 MiB | 487.8/505.2 MiB | 56.9/60.2 | 99.4/100.3 | — |
| `memory-heavy-1m` | Source | 223.1/232.6 | 385.5/386.6 MiB | 477.4/477.9 MiB | 0.0/0.0 | 33.1/35.8 | 61.6/148.3 |
| `structured-1m` | Write | 356.2/357.1 | 551.3/561.9 MiB | 646.1/663.1 MiB | 112.8/118.2 | 143.9/147.6 | — |
| `structured-1m` | Source | 350.9/352.7 | 517.4/519.2 MiB | 648.2/681.9 MiB | 0.0/0.1 | 32.4/37.6 | 66.0/71.0 |
| `table-heavy-1m` | Write | 70.5/74.3 | 212.2/217.2 MiB | 238.6/248.7 MiB | 0.1/0.3 | 33.2/34.5 | — |
| `table-heavy-1m` | Source | 71.3/74.5 | 235.8/239.2 MiB | 346.3/354.8 MiB | 0.0/0.0 | 28.5/34.3 | 58.8/182.6 |
| `table-long-cells-1m` | Write | 70.7/73.4 | 196.0/196.4 MiB | 236.7/255.8 MiB | 0.1/0.2 | 39.6/44.3 | — |
| `table-long-cells-1m` | Source | 69.8/75.4 | 192.5/194.0 MiB | 236.1/243.7 MiB | 0.0/0.0 | 35.1/37.7 | 55.9/73.0 |
| `target-shape-12x1m` | Write | 72.7/73.3 | 212.4/212.5 MiB | 238.7/242.6 MiB | 0.1/0.3 | 36.6/39.1 | — |
| `target-shape-12x1m` | Source | 68.8/78.4 | 229.1/237.9 MiB | 361.4/361.6 MiB | 0.0/0.0 | 34.7/38.7 | 57.6/228.5 |

## 关键结果

- 普通表格 fixture 的打开 action 约为 70 ms；`structured-1m` 的打开 action 约为 350 ms。
- `memory-heavy-1m` Write 的编辑 action 为 56.9/60.2，保存 action 为 99.4/100.3，RSS 峰值为 487.8/505.2 MiB。
- `structured-1m` Write 的编辑 action 为 112.8/118.2，保存 action 为 143.9/147.6，RSS 峰值为 646.1/663.1 MiB。
- 1 MiB 单行文档的 Write 请求自动转入 Source 模式，3 次运行均完成，RSS 峰值为 249.0/253.2 MiB，避免了原先的超时和内存暴涨。
- Source 模式对超长单行保持稳定，`long-line-1m` 的打开 action 为 59.2/60.7，加载后 RSS 为 198.7/198.8 MiB。

完整原始数据见 [`performance-data.jsonl`](performance-data.jsonl)。
