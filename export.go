package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 中文数字，用于输出文件名，如 01_一级视图.sql
var cnDigits = []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}

func cnNum(n int) string {
	switch {
	case n < 0:
		return fmt.Sprint(n)
	case n < 10:
		return cnDigits[n]
	case n == 10:
		return "十"
	case n < 20:
		return "十" + cnDigits[n%10]
	case n < 100:
		s := cnDigits[n/10] + "十"
		if r := n % 10; r > 0 {
			s += cnDigits[r]
		}
		return s
	case n < 1000:
		s := cnDigits[n/100] + "百"
		r := n % 100
		switch {
		case r == 0:
			return s
		case r < 10:
			return s + "零" + cnDigits[r]
		default:
			return s + cnNum(r)
		}
	default:
		return fmt.Sprint(n)
	}
}

// levelFileName 生成某一层级的输出文件名，例如 03_三级视图.sql
func levelFileName(level int) string {
	return fmt.Sprintf("%02d_%s级视图.sql", level, cnNum(level))
}

// exportLevels 按层级把视图原始 CREATE VIEW 语句写入独立 SQL 文件。
// 返回 生成的文件路径列表（按层级排序）。
func exportLevels(res *AnalyzeResult, outputDir string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}
	levels := make([]int, 0, len(res.LevelBuckets))
	for lv := range res.LevelBuckets {
		levels = append(levels, lv)
	}
	sort.Ints(levels)

	var files []string
	written := map[string]bool{}
	for _, lv := range levels {
		list := res.LevelBuckets[lv]
		var sb strings.Builder
		for _, v := range list {
			sb.WriteString(strings.TrimRight(v.Core, " \t\r\n"))
			sb.WriteString("\n")
		}
		name := levelFileName(lv)
		path := filepath.Join(outputDir, name)
		if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
			return files, fmt.Errorf("写入 %s 失败: %w", path, err)
		}
		written[name] = true
		files = append(files, path)
	}
	cleanStaleLevelFiles(outputDir, written)
	return files, nil
}

// tableFileName 物理表导出文件名
const tableFileName = "表.sql"

// exportTables 把所有物理表的原始 CREATE TABLE 语句导出到 表.sql。
// includeData 为 true 时，同时导出各表的 INSERT 数据（紧跟在建表语句之后）。
// 返回生成的文件路径（无表时返回空字符串，并清理过期文件）。
func exportTables(tables []*Table, outputDir string, includeData bool) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}
	path := filepath.Join(outputDir, tableFileName)
	if len(tables) == 0 {
		if err := os.Remove(path); err == nil {
			logf("无物理表，清理过期文件: %s", tableFileName)
		}
		return "", nil
	}
	var sb strings.Builder
	for _, t := range tables {
		sb.WriteString(strings.TrimRight(t.Core, " \t\r\n"))
		sb.WriteString("\n")
		if includeData {
			for _, d := range t.Data {
				sb.WriteString(strings.TrimRight(d, " \t\r\n"))
				sb.WriteString("\n")
			}
		}
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("写入 %s 失败: %w", path, err)
	}
	return path, nil
}

// cleanStaleLevelFiles 清理上一次运行遗留、本次未再生成的层级文件。
func cleanStaleLevelFiles(outputDir string, written map[string]bool) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if written[name] || !strings.HasSuffix(name, "视图.sql") {
			continue
		}
		base := strings.TrimSuffix(name, "视图.sql")
		if i := strings.Index(base, "_"); i == 2 && len(base) >= 3 {
			if err := os.Remove(filepath.Join(outputDir, name)); err == nil {
				logf("清理过期输出文件: %s", name)
			}
		}
	}
}
