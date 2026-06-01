# 参考资料

- VictoriaMetrics single-node 官方文档：https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/
- VictoriaMetrics quick start：https://docs.victoriametrics.com/victoriametrics/quick-start/
- SQLite PRAGMA 官方文档：https://www.sqlite.org/pragma.html
- SQLite VACUUM 官方文档：https://sqlite.org/lang_vacuum.html

选型结论：

- 当前 MVP 为了依赖最小化，使用 gzip JSONL 分段文件承载时序数据，使用 JSON 文件承载低频元数据。
- VictoriaMetrics single-node 依赖简单，具备时序压缩、保留策略和最小空闲空间保护，适合作为设备数量和查询复杂度提升后的时序后端。
- SQLite 适合保存低频元数据。配合 `auto_vacuum=INCREMENTAL`、`incremental_vacuum` 和定期 `VACUUM`，可以控制元数据文件膨胀。
