package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Tokenizer
// ---------------------------------------------------------------------------

type tokenKind int

const (
	tkWord   tokenKind = iota // 裸词：关键字 / 标识符 / 数字
	tkIdent                   // `反引号标识符`
	tkString                  // '字符串' 或 "字符串"
	tkPunct                   // 其它单字符
)

type token struct {
	kind  tokenKind
	text  string // word: 原文；ident: 去反引号；string: 去引号
	start int    // 在源串中的起始字节
	end   int    // 在源串中的结束字节（不含）
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\v' || c == '\f'
}

func isWordChar(c byte) bool {
	return c == '_' || c == '$' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// tokenize 切分 SQL 片段，跳过空白与注释（--、#、/* */，含 MySQL 条件注释 /*! */）。
func tokenize(s string) []token {
	var toks []token
	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case isSpace(c):
			i++
		case c == '-' && i+1 < n && s[i+1] == '-' && (i+2 >= n || isSpace(s[i+2])):
			for i < n && s[i] != '\n' {
				i++
			}
		case c == '#':
			for i < n && s[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && s[i+1] == '*':
			i += 2
			for i+1 < n && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
		case c == '`':
			start := i
			i++
			var sb strings.Builder
			for i < n {
				if s[i] == '`' {
					if i+1 < n && s[i+1] == '`' {
						sb.WriteByte('`')
						i += 2
						continue
					}
					i++
					break
				}
				sb.WriteByte(s[i])
				i++
			}
			toks = append(toks, token{kind: tkIdent, text: sb.String(), start: start, end: i})
		case c == '\'' || c == '"':
			start := i
			i++
			var sb strings.Builder
			for i < n {
				if s[i] == '\\' && i+1 < n {
					sb.WriteByte(s[i+1])
					i += 2
					continue
				}
				if s[i] == c {
					i++
					break
				}
				sb.WriteByte(s[i])
				i++
			}
			toks = append(toks, token{kind: tkString, text: sb.String(), start: start, end: i})
		case isWordChar(c):
			start := i
			for i < n && isWordChar(s[i]) {
				i++
			}
			toks = append(toks, token{kind: tkWord, text: s[start:i], start: start, end: i})
		default:
			toks = append(toks, token{kind: tkPunct, text: string(c), start: i, end: i + 1})
			i++
		}
	}
	return toks
}

// ---------------------------------------------------------------------------
// 语句切分
// ---------------------------------------------------------------------------

// splitStatements 按分号切分 SQL 文本，正确跳过字符串、反引号、注释。
// 返回的每条语句都包含结尾的分号。
func splitStatements(s string) []string {
	var out []string
	var b strings.Builder
	i, n := 0, len(s)
	push := func() {
		if strings.TrimSpace(b.String()) != "" {
			out = append(out, b.String())
		}
		b.Reset()
	}
	for i < n {
		c := s[i]
		switch {
		case c == '-' && i+1 < n && s[i+1] == '-' && (i+2 >= n || isSpace(s[i+2])):
			start := i
			for i < n && s[i] != '\n' {
				i++
			}
			b.WriteString(s[start:i])
		case c == '#':
			start := i
			for i < n && s[i] != '\n' {
				i++
			}
			b.WriteString(s[start:i])
		case c == '/' && i+1 < n && s[i+1] == '*':
			start := i
			i += 2
			for i+1 < n && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
			b.WriteString(s[start:i])
		case c == '\'' || c == '"':
			start := i
			i++
			for i < n {
				if s[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if s[i] == c {
					i++
					break
				}
				i++
			}
			b.WriteString(s[start:i])
		case c == '`':
			start := i
			i++
			for i < n {
				if s[i] == '`' {
					if i+1 < n && s[i+1] == '`' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			b.WriteString(s[start:i])
		case c == ';':
			b.WriteByte(';')
			push()
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	push()
	return out
}

// trimLeadingNoise 去掉语句开头的空白与注释，返回以 SQL 关键字开头的主体。
func trimLeadingNoise(s string) string {
	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case isSpace(c):
			i++
		case c == '-' && i+1 < n && s[i+1] == '-' && (i+2 >= n || isSpace(s[i+2])):
			for i < n && s[i] != '\n' {
				i++
			}
		case c == '#':
			for i < n && s[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && s[i+1] == '*':
			i += 2
			for i+1 < n && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
		default:
			return s[i:]
		}
	}
	return ""
}

func isCreateStatement(core string) bool {
	toks := tokenize(core)
	return len(toks) > 0 && toks[0].kind == tkWord && strings.EqualFold(toks[0].text, "CREATE")
}

// ---------------------------------------------------------------------------
// CREATE VIEW 解析
// ---------------------------------------------------------------------------

// errNotView 表示该 CREATE 语句不是视图（表、库、索引等），可安全忽略。
type notViewError struct{}

func (notViewError) Error() string { return "not a view" }

var nonViewKeywords = map[string]bool{
	"TABLE": true, "DATABASE": true, "SCHEMA": true, "INDEX": true,
	"UNIQUE": true, "FULLTEXT": true, "SPATIAL": true, "USER": true,
	"TRIGGER": true, "PROCEDURE": true, "FUNCTION": true, "EVENT": true,
	"ROLE": true,
}

func isPunct(t token, s string) bool { return t.kind == tkPunct && t.text == s }

// scanCreateKeyword 解析 CREATE 与对象类型关键字之间的可选属性
// （OR REPLACE / ALGORITHM= / DEFINER= / SQL SECURITY / TEMPORARY / UNIQUE ...）。
// 返回对象类型关键字（大写）及其后的 token 下标。
func scanCreateKeyword(toks []token, i int) (string, int, error) {
	for i < len(toks) {
		t := toks[i]
		if t.kind != tkWord {
			break
		}
		w := strings.ToUpper(t.text)
		switch w {
		case "OR", "REPLACE", "TEMPORARY", "UNIQUE", "FULLTEXT", "SPATIAL":
			i++
		case "ALGORITHM":
			i++
			if i < len(toks) && isPunct(toks[i], "=") {
				i++
			}
			if i < len(toks) && toks[i].kind == tkWord {
				i++
			}
		case "DEFINER":
			i++
			if i < len(toks) && isPunct(toks[i], "=") {
				i++
			}
			if i < len(toks) && (toks[i].kind == tkWord || toks[i].kind == tkIdent || toks[i].kind == tkString) {
				if strings.Contains(toks[i].text, "@") {
					i++
				} else {
					i++
					if i < len(toks) && isPunct(toks[i], "@") {
						i++
						if i < len(toks) && (toks[i].kind == tkWord || toks[i].kind == tkIdent || toks[i].kind == tkString) {
							i++
						}
					}
				}
			}
		case "SQL":
			i++ // SECURITY
			if i < len(toks) && toks[i].kind == tkWord && strings.EqualFold(toks[i].text, "SECURITY") {
				i++
			}
			if i < len(toks) && toks[i].kind == tkWord {
				i++ // DEFINER | INVOKER
			}
		default:
			return w, i + 1, nil // 视图/表/库等对象类型关键字
		}
	}
	return "", i, fmt.Errorf("CREATE 语句缺少对象类型关键字")
}

func isCreateKeyword(core string, kw string) bool {
	toks := tokenize(core)
	if len(toks) == 0 || toks[0].kind != tkWord || !strings.EqualFold(toks[0].text, "CREATE") {
		return false
	}
	got, _, err := scanCreateKeyword(toks, 1)
	return err == nil && strings.EqualFold(got, kw)
}

// parseCreateView 解析 CREATE VIEW 语句头部，返回库名、视图名、定义主体（AS 之后）。
func parseCreateView(core string) (db, name, body string, err error) {
	toks := tokenize(core)
	if len(toks) == 0 || toks[0].kind != tkWord || !strings.EqualFold(toks[0].text, "CREATE") {
		return "", "", "", fmt.Errorf("不是 CREATE 语句")
	}
	kw, i, err := scanCreateKeyword(toks, 1)
	if err != nil {
		return "", "", "", err
	}
	if !strings.EqualFold(kw, "VIEW") {
		if nonViewKeywords[kw] {
			return "", "", "", notViewError{} // CREATE TABLE/DATABASE/INDEX 等
		}
		return "", "", "", fmt.Errorf("CREATE 头部出现无法识别的关键字: %s", kw)
	}
	return parseViewTail(toks, i, core)
}

// parseCreateTable 解析 CREATE TABLE 语句，返回库名与表名。
func parseCreateTable(core string) (db, name string, ok bool) {
	toks := tokenize(core)
	if len(toks) == 0 || toks[0].kind != tkWord || !strings.EqualFold(toks[0].text, "CREATE") {
		return "", "", false
	}
	kw, i, err := scanCreateKeyword(toks, 1)
	if err != nil || !strings.EqualFold(kw, "TABLE") {
		return "", "", false
	}
	// 可选的 IF NOT EXISTS
	for i < len(toks) && toks[i].kind == tkWord {
		switch strings.ToUpper(toks[i].text) {
		case "IF", "NOT", "EXISTS":
			i++
		default:
			goto name
		}
	}

name:
	if i >= len(toks) || (toks[i].kind != tkWord && toks[i].kind != tkIdent) {
		return "", "", false
	}
	name = toks[i].text
	if i+2 < len(toks) && isPunct(toks[i+1], ".") &&
		(toks[i+2].kind == tkWord || toks[i+2].kind == tkIdent) {
		db = name
		name = toks[i+2].text
	}
	return db, name, true
}

// parseInsertTarget 提取 INSERT INTO `tbl` 的目标表名。
func parseInsertTarget(core string) string {
	toks := tokenize(core)
	if len(toks) == 0 || toks[0].kind != tkWord || !strings.EqualFold(toks[0].text, "INSERT") {
		return ""
	}
	for i := 1; i < len(toks); i++ {
		if toks[i].kind != tkWord || !strings.EqualFold(toks[i].text, "INTO") {
			continue
		}
		j := i + 1
		if j < len(toks) && (toks[j].kind == tkWord || toks[j].kind == tkIdent) {
			name := toks[j].text
			if j+2 < len(toks) && isPunct(toks[j+1], ".") &&
				(toks[j+2].kind == tkWord || toks[j+2].kind == tkIdent) {
				name = toks[j+2].text
			}
			return name
		}
	}
	return ""
}

// parseViewTail 解析 VIEW 关键字之后的名称与 AS 主体。
func parseViewTail(toks []token, i int, core string) (db, name, body string, err error) {
	if i >= len(toks) || (toks[i].kind != tkWord && toks[i].kind != tkIdent) {
		return "", "", "", fmt.Errorf("VIEW 后缺少视图名称")
	}
	name = toks[i].text
	if i+2 < len(toks) && isPunct(toks[i+1], ".") &&
		(toks[i+2].kind == tkWord || toks[i+2].kind == tkIdent) {
		db = name
		name = toks[i+2].text
		i += 2
	}
	i++
	// 可选的列清单 view_name (col1, col2)
	if i < len(toks) && isPunct(toks[i], "(") {
		depth := 0
		for i < len(toks) {
			if isPunct(toks[i], "(") {
				depth++
			} else if isPunct(toks[i], ")") {
				depth--
			}
			i++
			if depth == 0 {
				break
			}
		}
	}
	if i >= len(toks) || toks[i].kind != tkWord || !strings.EqualFold(toks[i].text, "AS") {
		return "", "", "", fmt.Errorf("视图 %s 缺少 AS 定义主体", name)
	}
	// 主体 = AS 关键字之后直到语句末尾的原文
	return db, name, core[toks[i].end:], nil
}

// ---------------------------------------------------------------------------
// 依赖提取
// ---------------------------------------------------------------------------

// extractTableRefs 从视图定义主体中提取 FROM / JOIN 后引用的对象名。
// 返回去重后的引用列表（含裸名与带库名的全名）。
func extractTableRefs(body string) []string {
	toks := tokenize(body)
	var refs []string
	seen := map[string]bool{}
	add := func(s string) {
		if s == "" || seen[strings.ToLower(s)] {
			return
		}
		seen[strings.ToLower(s)] = true
		refs = append(refs, s)
	}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind != tkWord {
			continue
		}
		u := strings.ToUpper(t.text)
		if u != "FROM" && u != "JOIN" {
			continue
		}
		// EXTRACT(YEAR FROM x) / SUBSTRING(x FROM y) / TRIM(x FROM y) 不是表引用
		if u == "FROM" && i > 0 && toks[i-1].kind == tkWord {
			switch strings.ToUpper(toks[i-1].text) {
			case "EXTRACT", "SUBSTRING", "TRIM":
				continue
			}
		}
		j := i + 1
		if j >= len(toks) {
			continue
		}
		if isPunct(toks[j], "(") {
			// 派生表 (select ...)，其内部的 FROM 会被单独扫描到
			continue
		}
		if toks[j].kind != tkWord && toks[j].kind != tkIdent {
			continue
		}
		bare := toks[j].text
		full := bare
		if j+2 < len(toks) && isPunct(toks[j+1], ".") &&
			(toks[j+2].kind == tkWord || toks[j+2].kind == tkIdent) {
			bare = toks[j+2].text
			full = full + "." + bare
			j += 2
		}
		add(bare)
		if full != bare {
			add(full)
		}
		i = j
	}
	return refs
}
