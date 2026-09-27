// MySQL 视图嵌套层级拆分导出工具
//
// 读取 MySQL 导出的 SQL 文件，解析其中所有 CREATE VIEW 语句，
// 自动识别视图嵌套层级（一级/二级/三级/...），按层级分别导出 SQL 文件。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Config 配置文件内容（文件名与主程序同名，后缀 .json）。
type Config struct {
	InputSQLPath         string `json:"input_sql_path"`        // 输入：MySQL 导出的 SQL 文件
	OutputDir            string `json:"output_dir"`            // 输出：各层级 sql 文件目录
	EnableLog            bool   `json:"enable_log"`            // 是否打印控制台日志
	IncludeTableData     bool   `json:"include_table_data"`    // 导出表时是否同时导出 INSERT 数据
	DisableTableExport   bool   `json:"disable_table_export"`  // 是否禁用物理表导出（表.sql）
}

var (
	cfg     Config
	errLogs []string
)

func logf(format string, a ...any) {
	if !cfg.EnableLog {
		return
	}
	fmt.Printf(format+"\n", a...)
}

func warnf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	errLogs = append(errLogs, "[WARN] "+msg)
	fmt.Fprintln(os.Stderr, "[WARN] "+msg)
}

func errf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	errLogs = append(errLogs, "[ERROR] "+msg)
	fmt.Fprintln(os.Stderr, "[ERROR] "+msg)
}

func main() {
	configPath, cfgErr := resolveConfigPath()

	var raw []byte
	var err error
	if cfgErr == nil {
		raw, err = os.ReadFile(configPath)
		if err != nil {
			cfgErr = fmt.Errorf("读取配置文件 %s 失败: %w", configPath, err)
		}
	}
	if cfgErr == nil {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			cfgErr = fmt.Errorf("配置文件 %s JSON 解析失败: %w", configPath, err)
		}
	}
	if cfgErr != nil {
		errf("%v", cfgErr)
		flushErrorLog("")
		os.Exit(1)
	}

	if cfg.OutputDir == "" {
		cfg.OutputDir = "view_output"
	}

	logf("配置文件: %s", configPath)
	logf("输入 SQL 文件: %s", cfg.InputSQLPath)
	logf("输出目录: %s", cfg.OutputDir)

	if cfg.InputSQLPath == "" {
		errf("配置项 input_sql_path 为空，请在 %s 中指定输入 SQL 文件路径", configPath)
		flushErrorLog(cfg.OutputDir)
		os.Exit(1)
	}

	// 1. 读取 SQL 文件
	data, err := os.ReadFile(cfg.InputSQLPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			errf("找不到 SQL 文件: %s", cfg.InputSQLPath)
		} else {
			errf("读取 SQL 文件失败: %v", err)
		}
		flushErrorLog(cfg.OutputDir)
		os.Exit(1)
	}
	text := stripBOM(string(data))
	if !utf8.ValidString(text) {
		warnf("SQL 文件包含非法 UTF-8 字符，建议转码为 UTF-8 后重试")
	}

	// 2. 解析 CREATE VIEW / CREATE TABLE 语句
	views, tables := parseSQL(text)
	if len(views) == 0 && len(tables) == 0 {
		errf("SQL 文件中未解析到任何 CREATE VIEW / CREATE TABLE 语句: %s", cfg.InputSQLPath)
		flushErrorLog(cfg.OutputDir)
		os.Exit(1)
	}
	// 过滤 HeidiSQL 为解决视图依赖生成的同名临时表
	tables, skipped := filterTempTables(tables, views)
	logf("解析到视图语句 %d 条，表语句 %d 条（跳过视图同名临时表 %d 个）", len(views), len(tables), skipped)
	if len(views) == 0 {
		warnf("SQL 文件中未解析到任何 CREATE VIEW 视图语句: %s", cfg.InputSQLPath)
	}

	// 3. 依赖分析 & 层级计算
	res := Analyze(views)

	// 4. 按层级导出视图
	files, err := exportLevels(res, cfg.OutputDir)
	if err != nil {
		errf("%v", err)
		flushErrorLog(cfg.OutputDir)
		os.Exit(1)
	}

	// 5. 导出物理表
	tableFile := ""
	if !cfg.DisableTableExport {
		tableFile, err = exportTables(tables, cfg.OutputDir, cfg.IncludeTableData)
		if err != nil {
			errf("%v", err)
			flushErrorLog(cfg.OutputDir)
			os.Exit(1)
		}
	}

	// 6. 统计日志
	printStats(res, files, tableFile)
	flushErrorLog(cfg.OutputDir)
}

// Table 一个被解析出来的物理表。
type Table struct {
	Db   string   // 库名（可能为空）
	Name string   // 表名
	Core string   // 原始 CREATE TABLE 语句（保留原文）
	Data []string // 该表的 INSERT 语句（启用 include_table_data 时导出）
}

// parseSQL 从 SQL 文本中提取所有 CREATE VIEW 与 CREATE TABLE 语句，
// 并把 INSERT 数据归属到对应表上。
func parseSQL(text string) (views []*View, tables []*Table) {
	insertOwner := map[string]*Table{}
	var pendingInserts []pendingInsert // 出现在所属表之前的 INSERT

	for _, stmt := range splitStatements(text) {
		core := trimLeadingNoise(stmt)
		if core == "" {
			continue
		}
		if !isCreateStatement(core) {
			// INSERT INTO `tbl` ... 数据（可能出现在所属表之前，之后再回填）
			if cfg.IncludeTableData {
				if target := parseInsertTarget(core); target != "" {
					if owner, ok := insertOwner[normalizeKey(target)]; ok {
						owner.Data = append(owner.Data, core)
					} else {
						pendingInserts = append(pendingInserts, pendingInsert{target: normalizeKey(target), stmt: core})
					}
				}
			}
			continue
		}
		db, name, body, err := parseCreateView(core)
		if err == nil {
			views = append(views, &View{
				Name:   name,
				Db:     db,
				Core:   core,
				Refs:   extractTableRefs(body),
				Status: StatusOK,
			})
			continue
		}
		var nv notViewError
		if !errors.As(err, &nv) {
			head := core
			if len(head) > 120 {
				head = head[:120] + "..."
			}
			errf("SQL 语法解析失败: %v；语句: %s", err, strings.ReplaceAll(head, "\n", " "))
			continue
		}
		// 不是视图 → 尝试按表解析
		if tdb, tname, ok := parseCreateTable(core); ok {
			t := &Table{Db: tdb, Name: tname, Core: core}
			tables = append(tables, t)
			insertOwner[normalizeKey(tname)] = t
		}
	}

	// 处理出现在所属表之前的 INSERT
	if len(pendingInserts) > 0 {
		for _, p := range pendingInserts {
			if owner, ok := insertOwner[p.target]; ok {
				owner.Data = append(owner.Data, p.stmt)
			}
		}
		pendingInserts = nil
	}
	return dedupViews(views), tables
}

type pendingInsert struct {
	target string
	stmt   string
}

func normalizeKey(s string) string { return strings.ToLower(s) }

// filterTempTables 去掉与视图同名的临时表（HeidiSQL 为绕过视图依赖顺序而生成）。
func filterTempTables(tables []*Table, views []*View) ([]*Table, int) {
	viewNames := make(map[string]bool, len(views))
	for _, v := range views {
		viewNames[normalizeKey(v.Name)] = true
	}
	out := make([]*Table, 0, len(tables))
	skipped := 0
	for _, t := range tables {
		if viewNames[normalizeKey(t.Name)] {
			skipped++
			continue
		}
		out = append(out, t)
	}
	return out, skipped
}

// dedupViews 同名视图（重复定义）只保留最后一个
func dedupViews(views []*View) []*View {
	uniq := make(map[string]*View, len(views))
	var order []string
	for _, v := range views {
		key := normalizeKey(v.Name)
		if _, ok := uniq[key]; !ok {
			order = append(order, key)
		}
		uniq[key] = v
	}
	out := make([]*View, 0, len(uniq))
	for _, k := range order {
		out = append(out, uniq[k])
	}
	return out
}

func printStats(res *AnalyzeResult, files []string, tableFile string) {
	logf("")
	logf("================ 视图依赖分析统计 ================")
	logf("总视图数量: %d", len(res.Views))

	levels := make([]int, 0, len(res.LevelBuckets))
	for lv := range res.LevelBuckets {
		levels = append(levels, lv)
	}
	sort.Ints(levels)
	for _, lv := range levels {
		list := res.LevelBuckets[lv]
		logf("%s级视图 (L%d): %d 个 → %s", cnNum(lv), lv, len(list), levelFileName(lv))
		for _, v := range list {
			deps := "-"
			if len(v.Deps) > 0 {
				deps = strings.Join(v.Deps, ", ")
			}
			logf("    - %s (依赖: %s)", v.Name, deps)
		}
	}

	if len(res.CycleGroups) == 0 {
		logf("循环依赖视图: 无")
	} else {
		warnf("循环依赖视图 %d 组（无法计算层级，未写入层级文件）:", len(res.CycleGroups))
		for i, g := range res.CycleGroups {
			warnf("  第 %d 组: %s", i+1, strings.Join(g, " → ")+" → "+g[0])
		}
	}
	if len(res.Affected) > 0 {
		warnf("受循环依赖影响、无法计算层级的视图: %s", strings.Join(res.Affected, ", "))
	}

	// 引用到的物理表信息
	physSet := map[string]bool{}
	var physical []string
	for _, v := range res.Views {
		for _, u := range v.UnknownRefs {
			if !physSet[u] {
				physSet[u] = true
				physical = append(physical, u)
			}
		}
	}
	if len(physical) > 0 {
		sort.Strings(physical)
		logf("引用的物理表/外部对象: %s", strings.Join(physical, ", "))
	}

	if len(files) > 0 {
		logf("视图输出文件 %d 个:", len(files))
		for _, f := range files {
			logf("    %s", f)
		}
	}
	if tableFile != "" {
		logf("物理表输出文件: %s", tableFile)
	}
	logf("==================================================")
}

// resolveConfigPath 定位配置文件：-config 参数 > 主程序同名.json > view_parser.json
func resolveConfigPath() (string, error) {
	explicit := flag.String("config", "", "配置文件路径（默认与主程序同名.json）")
	flag.Parse()
	if *explicit != "" {
		if _, err := os.Stat(*explicit); err != nil {
			return "", fmt.Errorf("找不到配置文件: %s", *explicit)
		}
		return *explicit, nil
	}

	var candidates []string
	add := func(p string) {
		if p == "" {
			return
		}
		ext := filepath.Ext(p)
		c := strings.TrimSuffix(p, ext) + ".json"
		for _, exist := range candidates {
			if exist == c {
				return
			}
		}
		candidates = append(candidates, c)
	}
	if exe, err := os.Executable(); err == nil {
		add(exe)
	}
	add(os.Args[0])
	add(filepath.Join(".", "view_parser"))

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("找不到配置文件，期望路径之一: %s", strings.Join(candidates, " / "))
}

// flushErrorLog 若存在错误日志，写入 <outputDir>/error.log
func flushErrorLog(outputDir string) {
	if len(errLogs) == 0 {
		return
	}
	var sb strings.Builder
	sb.WriteString("# 错误日志 " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	for _, l := range errLogs {
		sb.WriteString(l + "\n")
	}
	if outputDir == "" {
		return
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] 创建输出目录失败，错误日志未落盘: "+err.Error())
		return
	}
	path := filepath.Join(outputDir, "error.log")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] 写入错误日志失败: "+err.Error())
		return
	}
	fmt.Fprintln(os.Stderr, "[ERROR] 错误日志已写入: "+path)
}

func stripBOM(s string) string {
	return strings.TrimPrefix(s, "\ufeff")
}
