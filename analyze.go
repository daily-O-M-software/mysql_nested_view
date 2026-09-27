package main

import (
	"sort"
	"strings"
)

// 视图状态
const (
	StatusOK       = "ok"       // 层级计算成功
	StatusCycle    = "cycle"    // 处于循环依赖中
	StatusAffected = "affected" // 依赖了循环依赖视图，无法计算层级
)

// View 一个被解析出来的视图。
type View struct {
	Name        string   // 视图名（不含库名）
	Db          string   // 库名（可能为空）
	Core        string   // 原始 CREATE VIEW 语句（保留原文）
	Refs        []string // 定义体中引用到的对象名（裸名 + 全名）
	Deps        []string // 其中属于本文件内视图的依赖（已解析为视图名）
	UnknownRefs []string // 引用到的非视图对象（物理表等）
	Level       int      // 嵌套层级：1=一级视图 ...
	Status      string   // StatusOK / StatusCycle / StatusAffected
}

// AnalyzeResult 依赖分析结果。
type AnalyzeResult struct {
	Views        []*View
	ByName       map[string]*View
	CycleGroups  [][]string // 循环依赖分组（每组内形成环）
	Affected     []string   // 依赖了循环依赖的视图名
	LevelBuckets map[int][]*View
}

// Analyze 构建依赖图、识别循环依赖并计算每个视图的嵌套层级。
func Analyze(views []*View) *AnalyzeResult {
	res := &AnalyzeResult{
		Views:        views,
		ByName:       make(map[string]*View, len(views)),
		LevelBuckets: map[int][]*View{},
	}

	// 名称索引：精确匹配 + 忽略大小写匹配
	byLower := make(map[string]*View, len(views))
	for _, v := range views {
		res.ByName[v.Name] = v
		if _, ok := byLower[strings.ToLower(v.Name)]; !ok {
			byLower[strings.ToLower(v.Name)] = v
		}
	}
	resolve := func(ref string) *View {
		if v, ok := res.ByName[ref]; ok {
			return v
		}
		// 库名限定的引用：db.view
		if idx := strings.LastIndex(ref, "."); idx >= 0 {
			if v, ok := res.ByName[ref[idx+1:]]; ok {
				return v
			}
		}
		return byLower[strings.ToLower(ref)]
	}

	// 解析每条视图的依赖
	for _, v := range views {
		depSet := map[string]bool{}
		unknownSet := map[string]bool{}
		for _, ref := range v.Refs {
			if dep := resolve(ref); dep != nil {
				if !depSet[dep.Name] {
					depSet[dep.Name] = true
					v.Deps = append(v.Deps, dep.Name)
				}
			} else {
				key := ref
				if idx := strings.LastIndex(key, "."); idx >= 0 {
					key = key[idx+1:]
				}
				if !unknownSet[strings.ToLower(key)] {
					unknownSet[strings.ToLower(key)] = true
					v.UnknownRefs = append(v.UnknownRefs, key)
				}
			}
		}
	}

	// DFS 计算层级 + 识别循环依赖
	const (
		stNew       = 0
		stVisiting  = 1
		stDone      = 2
	)
	state := make(map[string]int, len(views))
	levelMemo := make(map[string]int, len(views))
	okMemo := make(map[string]bool, len(views))
	var stack []string
	pos := make(map[string]int)
	seenGroup := map[string]bool{}

	var dfs func(name string) (int, bool)
	dfs = func(name string) (int, bool) {
		switch state[name] {
		case stDone:
			return levelMemo[name], okMemo[name]
		case stVisiting:
			// 回到栈上节点 → 发现循环依赖，记录环上的所有节点
			g := append([]string{}, stack[pos[name]:]...)
			sort.Strings(g)
			key := strings.Join(g, ",")
			if !seenGroup[key] {
				seenGroup[key] = true
				res.CycleGroups = append(res.CycleGroups, g)
			}
			return 0, false
		}
		v := res.ByName[name]
		state[name] = stVisiting
		pos[name] = len(stack)
		stack = append(stack, name)

		resolvable := true
		maxLevel := 0
		if len(v.Deps) == 0 {
			v.Level = 1 // 只引用物理表 → 一级视图
		} else {
			for _, d := range v.Deps {
				lv, ok := dfs(d)
				if !ok {
					resolvable = false
					continue
				}
				if lv > maxLevel {
					maxLevel = lv
				}
			}
			if resolvable {
				v.Level = maxLevel + 1 // 取最深依赖层级 + 1
			}
		}

		stack = stack[:pos[name]]
		delete(pos, name)
		state[name] = stDone
		levelMemo[name] = v.Level
		okMemo[name] = resolvable
		if !resolvable {
			v.Level = 0
		}
		return v.Level, resolvable
	}

	// 稳定顺序遍历
	names := make([]string, 0, len(views))
	for _, v := range views {
		names = append(names, v.Name)
	}
	sort.Strings(names)

	cycleNodes := map[string]bool{}
	// 先跑一遍 DFS 收集环信息（环在回边处被记录）
	for _, n := range names {
		if state[n] == stNew {
			dfs(n)
		}
	}
	for _, g := range res.CycleGroups {
		for _, n := range g {
			cycleNodes[n] = true
		}
	}
	// 标记状态
	for _, v := range views {
		switch {
		case cycleNodes[v.Name]:
			v.Status = StatusCycle
			v.Level = 0
		case okMemo[v.Name]:
			v.Status = StatusOK
		default:
			v.Status = StatusAffected
			v.Level = 0
			res.Affected = append(res.Affected, v.Name)
		}
	}
	sort.Strings(res.Affected)

	// 按层级归档
	for _, v := range views {
		if v.Status == StatusOK {
			res.LevelBuckets[v.Level] = append(res.LevelBuckets[v.Level], v)
		}
	}
	for lv := range res.LevelBuckets {
		list := res.LevelBuckets[lv]
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	}
	return res
}
