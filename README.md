# MySQL 视图嵌套层级拆分导出工具

按 `rd.md` 需求实现：读取 MySQL 导出的 SQL 文件，解析所有 `CREATE VIEW` 语句，
分析视图依赖关系并计算嵌套层级，按层级分别导出 SQL 文件。

## 构建

```powershell
& "D:\alantop_dir\alantop_sde\go1.25\bin\go.exe" build -o view_parser.exe .
```

## 运行

配置文件与主程序同名（`view_parser.exe` → `view_parser.json`），放在程序同目录：

```json
{
  "input_sql_path": "0925.sql",
  "output_dir": "view_output",
  "enable_log": true,
  "include_table_data": false,
  "disable_table_export": false
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| input_sql_path | string | 输入：MySQL 导出的 SQL 文件完整路径 |
| output_dir | string | 输出目录，各层级 sql 文件存放目录 |
| enable_log | bool | 是否打印控制台日志 |
| include_table_data | bool | 导出表时是否同时导出 INSERT 数据（默认 false，只导出建表语句） |
| disable_table_export | bool | 是否禁用物理表导出（默认 false，即导出 `表.sql`） |

```powershell
.\view_parser.exe
# 或显式指定配置文件
.\view_parser.exe -config D:\demo\view_parser.json
```

## 输出

- `01_一级视图.sql`、`02_二级视图.sql`、`03_三级视图.sql`…… 每个文件只含对应层级视图的原始 `CREATE VIEW` 语句（原文保留，不改语法）；
- `表.sql`：所有物理表的原始 `CREATE TABLE` 语句（原文保留）；HeidiSQL 为绕过视图依赖生成的**与视图同名的临时表自动跳过**；
  配置 `include_table_data: true` 时，各表的 `INSERT` 数据会跟在建表语句之后一并导出；
- `error.log`（存在异常时）：SQL 解析失败、循环依赖等错误日志；
- 控制台统计：总视图数、各层级视图数与依赖、物理表数量、循环依赖视图列表。

## 层级规则

1. 视图只引用物理表 → L1（一级）；引用 L1 视图 → L2；引用 L2 → L3，以此类推。
2. 同时引用物理表和视图时按**最深依赖**判定（例如物理表 + L1 → L2）。
3. 循环依赖（A→B→A）无法计算层级：标记告警、写入 `error.log`、不写入层级文件；
   依赖循环依赖的视图同样无法计算层级，单独告警。
4. 只解析 `CREATE VIEW`，忽略 `CREATE TABLE/DATABASE/INDEX`、存储过程等其它 DDL。

## 代码结构

| 文件 | 说明 |
| --- | --- |
| `view_parser.go` | 入口：配置读取、流程编排、统计与错误日志 |
| `sqlparse.go` | 语句切分、词法解析、`CREATE VIEW`/`CREATE TABLE`/`INSERT` 解析、FROM/JOIN 依赖提取 |
| `analyze.go` | 依赖图构建、循环依赖识别（DFS）、层级计算 |
| `export.go` | 按层级导出视图、导出 `表.sql`、中文序号文件名、过期文件清理 |
