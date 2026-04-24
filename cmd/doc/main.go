package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HandlerInfo struct {
	Package    string
	TypeName   string
	Methods    []MethodInfo
	FilePath   string
}

type MethodInfo struct {
	Name    string
	Doc     string
	Params  []ParamInfo
	Results []ParamInfo
}

type ParamInfo struct {
	Name string
	Type string
	Docs string
}

type RemoteInfo struct {
	Package    string
	TypeName   string
	Methods    []MethodInfo
	FilePath   string
}

type FilterInfo struct {
	Package    string
	TypeName   string
	Before     bool
	After      bool
	FilePath   string
}

type CronInfo struct {
	Package    string
	TypeName   string
	Methods    []MethodInfo
	FilePath   string
}

type ServerType struct {
	Name     string
	Handlers []HandlerInfo
	Remotes  []RemoteInfo
	Filters  []FilterInfo
	Crons    []CronInfo
}

func main() {
	basePath, docPath, err := validateAndGetPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	types, err := scanServerTypes(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning server types: %v\n", err)
		os.Exit(1)
	}

	if len(types) == 0 {
		fmt.Fprintf(os.Stderr, "Error: 未找到任何 API 文档，请确保 servers/ 目录下有 handler/remote/cron/filter\n")
		os.Exit(1)
	}

	if err := os.MkdirAll(docPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating doc directory: %v\n", err)
		os.Exit(1)
	}

	generateHandlerDoc(types, filepath.Join(docPath, "handler.md"))
	generateRemoteDoc(types, filepath.Join(docPath, "remote.md"))
	generateCronDoc(types, filepath.Join(docPath, "cron.md"))
	generateFilterDoc(types, filepath.Join(docPath, "filter.md"))

	fmt.Println("文档已生成到 ./doc/ 目录")
}

func validateAndGetPaths() (basePath string, docPath string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("无法获取当前目录: %v", err)
	}

	if len(os.Args) > 1 {
		basePath = os.Args[1]
		if !filepath.IsAbs(basePath) {
			basePath = filepath.Join(cwd, basePath)
		}
	} else {
		basePath = filepath.Join(cwd, "servers")
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			return "", "", fmt.Errorf("请在 game-server 目录下运行 gomelo doc，或指定路径: gomelo doc ./servers")
		}
	}

	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("servers 目录不存在: %s", basePath)
	}

	serversDir, err := os.Open(basePath)
	if err != nil {
		return "", "", fmt.Errorf("无法打开 servers 目录: %v", err)
	}
	defer serversDir.Close()

	entries, _ := serversDir.ReadDir(0)
	hasSubDirs := false
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(basePath, entry.Name())
			for _, sub := range []string{"handler", "remote", "filter", "cron"} {
				if _, err := os.Stat(filepath.Join(subPath, sub)); err == nil {
					hasSubDirs = true
					break
				}
			}
		}
	}
	if !hasSubDirs {
		return "", "", fmt.Errorf("servers 目录下未找到 handler/remote/cron/filter 目录")
	}

	docPath = filepath.Join(cwd, "doc")
	return basePath, docPath, nil
}

func scanServerTypes(basePath string) ([]ServerType, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}

	var types []ServerType
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		serverType := entry.Name()
		st := ServerType{Name: serverType}

		handlerPath := filepath.Join(basePath, serverType, "handler")
		if info, err := os.Stat(handlerPath); err == nil && info.IsDir() {
			handlers, _ := scanHandlers(handlerPath, basePath, serverType)
			st.Handlers = handlers
		}

		remotePath := filepath.Join(basePath, serverType, "remote")
		if info, err := os.Stat(remotePath); err == nil && info.IsDir() {
			remotes, _ := scanRemotes(remotePath, basePath, serverType)
			st.Remotes = remotes
		}

		filterPath := filepath.Join(basePath, serverType, "filter")
		if info, err := os.Stat(filterPath); err == nil && info.IsDir() {
			filters, _ := scanFilters(filterPath, basePath, serverType)
			st.Filters = filters
		}

		cronPath := filepath.Join(basePath, serverType, "cron")
		if info, err := os.Stat(cronPath); err == nil && info.IsDir() {
			crons, _ := scanCrons(cronPath, basePath, serverType)
			st.Crons = crons
		}

		if len(st.Handlers) > 0 || len(st.Remotes) > 0 || len(st.Filters) > 0 || len(st.Crons) > 0 {
			types = append(types, st)
		}
	}

	return types, nil
}

func scanHandlers(dir string, basePath string, serverType string) ([]HandlerInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var allHandlers []HandlerInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		info, _ := parseHandlerFile(filePath, basePath, serverType)
		if info != nil && len(info.Methods) > 0 {
			allHandlers = append(allHandlers, *info)
		}
	}

	return allHandlers, nil
}

func parseHandlerFile(filePath string, basePath string, serverType string) (*HandlerInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	pkgName := node.Name.Name
	if pkgName == "" {
		pkgName = filepath.Base(filepath.Dir(filePath))
	}

	var handlers []HandlerInfo
	var currentType string
	var methods []MethodInfo

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil {
			continue
		}

		typeName := getTypeName(funcDecl.Recv.List[0].Type)
		if typeName == "" {
			continue
		}

		if !strings.HasSuffix(typeName, "Handler") && !strings.HasSuffix(typeName, "handler") {
			continue
		}

		if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) != 1 {
			continue
		}

		if !isContextParam(funcDecl.Type.Params.List[0]) {
			continue
		}

		if currentType != "" && currentType != typeName {
			handlers = append(handlers, HandlerInfo{
				Package:  pkgName,
				TypeName: currentType,
				Methods:  methods,
				FilePath: filePath,
			})
			methods = nil
		}

		methodInfo := parseMethodInfo(funcDecl)
		methods = append(methods, methodInfo)
		currentType = typeName
	}

	if currentType != "" && len(methods) > 0 {
		handlers = append(handlers, HandlerInfo{
			Package:  pkgName,
			TypeName: currentType,
			Methods:  methods,
			FilePath: filePath,
		})
	}

	if len(handlers) == 0 {
		return nil, nil
	}

	return &handlers[0], nil
}

func scanRemotes(dir string, basePath string, serverType string) ([]RemoteInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var remotes []RemoteInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		info, _ := parseRemoteFile(filePath, basePath, serverType)
		if info != nil && len(info.Methods) > 0 {
			remotes = append(remotes, *info)
		}
	}

	return remotes, nil
}

func parseRemoteFile(filePath string, basePath string, serverType string) (*RemoteInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	pkgName := node.Name.Name
	if pkgName == "" {
		pkgName = filepath.Base(filepath.Dir(filePath))
	}

	var remotes []RemoteInfo
	var currentType string
	var methods []MethodInfo

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil {
			continue
		}

		typeName := getTypeName(funcDecl.Recv.List[0].Type)
		if typeName == "" {
			continue
		}

		if !strings.HasSuffix(typeName, "Remote") && !strings.HasSuffix(typeName, "remote") {
			continue
		}

		if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) != 2 {
			continue
		}

		if !isContextParam(funcDecl.Type.Params.List[0]) {
			continue
		}

		if currentType != "" && currentType != typeName {
			remotes = append(remotes, RemoteInfo{
				Package:  pkgName,
				TypeName: currentType,
				Methods:  methods,
				FilePath: filePath,
			})
			methods = nil
		}

		methodInfo := parseMethodInfo(funcDecl)
		methods = append(methods, methodInfo)
		currentType = typeName
	}

	if currentType != "" && len(methods) > 0 {
		remotes = append(remotes, RemoteInfo{
			Package:  pkgName,
			TypeName: currentType,
			Methods:  methods,
			FilePath: filePath,
		})
	}

	if len(remotes) == 0 {
		return nil, nil
	}

	return &remotes[0], nil
}

func scanFilters(dir string, basePath string, serverType string) ([]FilterInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var allFilters []FilterInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		info, _ := parseFilterFile(filePath, basePath, serverType)
		if info != nil {
			allFilters = append(allFilters, *info)
		}
	}

	return allFilters, nil
}

func parseFilterFile(filePath string, basePath string, serverType string) (*FilterInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	pkgName := node.Name.Name
	if pkgName == "" {
		pkgName = filepath.Base(filepath.Dir(filePath))
	}

	var filters []FilterInfo
	var currentType string
	var hasBefore, hasAfter bool

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil {
			continue
		}

		typeName := getTypeName(funcDecl.Recv.List[0].Type)
		if typeName == "" {
			continue
		}

		if !strings.HasSuffix(typeName, "Filter") && !strings.HasSuffix(typeName, "filter") {
			continue
		}

		if currentType != "" && currentType != typeName {
			filters = append(filters, FilterInfo{
				Package:  pkgName,
				TypeName: currentType,
				Before:   hasBefore,
				After:    hasAfter,
				FilePath: filePath,
			})
			hasBefore = false
			hasAfter = false
		}

		if funcDecl.Name.Name == "Before" {
			hasBefore = true
		} else if funcDecl.Name.Name == "After" {
			hasAfter = true
		}
		currentType = typeName
	}

	if currentType != "" {
		filters = append(filters, FilterInfo{
			Package:  pkgName,
			TypeName: currentType,
			Before:   hasBefore,
			After:    hasAfter,
			FilePath: filePath,
		})
	}

	if len(filters) == 0 {
		return nil, nil
	}

	return &filters[0], nil
}

func scanCrons(dir string, basePath string, serverType string) ([]CronInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var allCrons []CronInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		info, _ := parseCronFile(filePath, basePath, serverType)
		if info != nil && len(info.Methods) > 0 {
			allCrons = append(allCrons, *info)
		}
	}

	return allCrons, nil
}

func parseCronFile(filePath string, basePath string, serverType string) (*CronInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	pkgName := node.Name.Name
	if pkgName == "" {
		pkgName = filepath.Base(filepath.Dir(filePath))
	}

	var crons []CronInfo
	var currentType string
	var methods []MethodInfo

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil {
			continue
		}

		typeName := getTypeName(funcDecl.Recv.List[0].Type)
		if typeName == "" {
			continue
		}

		if !strings.HasSuffix(typeName, "Cron") && !strings.HasSuffix(typeName, "cron") {
			continue
		}

		if funcDecl.Type.Params != nil && len(funcDecl.Type.Params.List) > 1 {
			continue
		}

		if currentType != "" && currentType != typeName {
			crons = append(crons, CronInfo{
				Package:  pkgName,
				TypeName: currentType,
				Methods:  methods,
				FilePath: filePath,
			})
			methods = nil
		}

		methodInfo := parseMethodInfo(funcDecl)
		methods = append(methods, methodInfo)
		currentType = typeName
	}

	if currentType != "" && len(methods) > 0 {
		crons = append(crons, CronInfo{
			Package:  pkgName,
			TypeName: currentType,
			Methods:  methods,
			FilePath: filePath,
		})
	}

	if len(crons) == 0 {
		return nil, nil
	}

	return &crons[0], nil
}

func getTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func isContextParam(field *ast.Field) bool {
	if field.Type == nil {
		return false
	}

	switch t := field.Type.(type) {
	case *ast.Ident:
		return t.Name == "Context"
	case *ast.StarExpr:
		if sel, ok := t.X.(*ast.SelectorExpr); ok {
			if sel.Sel != nil && sel.Sel.Name == "Context" {
				return true
			}
		}
	case *ast.SelectorExpr:
		if t.Sel != nil && t.Sel.Name == "Context" {
			return true
		}
	}
	return false
}

func parseMethodInfo(funcDecl *ast.FuncDecl) MethodInfo {
	method := MethodInfo{
		Name: funcDecl.Name.Name,
	}

	if funcDecl.Doc != nil {
		method.Doc = strings.TrimSpace(funcDecl.Doc.Text())
	}

	if funcDecl.Type.Params != nil {
		for _, param := range funcDecl.Type.Params.List {
			paramName := getParamName(param.Names)
			paramType := getTypeString(param.Type)
			method.Params = append(method.Params, ParamInfo{
				Name: paramName,
				Type: paramType,
			})
		}
	}

	if funcDecl.Type.Results != nil {
		for _, result := range funcDecl.Type.Results.List {
			resultType := getTypeString(result.Type)
			method.Results = append(method.Results, ParamInfo{
				Type: resultType,
			})
		}
	}

	return method
}

func getParamName(names []*ast.Ident) string {
	if len(names) > 0 {
		return names[0].Name
	}
	return ""
}

func getTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + getTypeString(t.X)
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name + "." + t.Sel.Name
		}
	case *ast.ArrayType:
		return "[]" + getTypeString(t.Elt)
	case *ast.MapType:
		return "map[" + getTypeString(t.Key) + "]" + getTypeString(t.Value)
	case *ast.InterfaceType:
		if t.Methods != nil && len(t.Methods.List) == 0 {
			return "any"
		}
	case *ast.FuncType:
		return "function"
	}
	return "any"
}

func generateHandlerDoc(types []ServerType, path string) {
	buf := &strings.Builder{}
	buf.WriteString("# Handler API\n\n")
	buf.WriteString("> 本文档由 `gomelo doc` 自动生成，")
	buf.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	buf.WriteString("\n\n")

	buf.WriteString("## 目录\n\n")
	buf.WriteString("- [Handler](#handler)\n")
	buf.WriteString("- [Remote](#remote)\n")
	buf.WriteString("- [Cron](#cron)\n")
	buf.WriteString("- [Filter](#filter)\n\n")

	buf.WriteString("---\n\n")

	for _, st := range types {
		if len(st.Handlers) == 0 {
			continue
		}

		buf.WriteString("## ")
		buf.WriteString(st.Name)
		buf.WriteString("\n\n")

		for _, h := range st.Handlers {
			buf.WriteString("### ")
			buf.WriteString(h.TypeName)
			buf.WriteString("\n\n")

			buf.WriteString("| 路由 | 方法 | 描述 |\n")
			buf.WriteString("|------|------|------|\n")

			for _, m := range h.Methods {
				route := strings.ToLower(st.Name + "." + m.Name)
				desc := m.Doc
				if desc == "" {
					desc = "-"
				}
				buf.WriteString("| ")
				buf.WriteString(route)
				buf.WriteString(" | ")
				buf.WriteString(m.Name)
				buf.WriteString(" | ")
				buf.WriteString(desc)
				buf.WriteString(" |\n")
			}

			buf.WriteString("\n")

			for _, m := range h.Methods {
				route := strings.ToLower(st.Name + "." + m.Name)
				buf.WriteString("#### ")
				buf.WriteString(route)
				buf.WriteString("\n\n")

				if m.Doc != "" {
					buf.WriteString(m.Doc)
					buf.WriteString("\n\n")
				}

				if len(m.Params) > 1 {
					buf.WriteString("**请求参数**:\n\n")
					buf.WriteString("| 字段 | 类型 | 说明 |\n")
					buf.WriteString("|------|------|------|\n")
					for i, p := range m.Params {
						if i == 0 {
							continue
						}
						buf.WriteString("| ")
						buf.WriteString(p.Name)
						buf.WriteString(" | ")
						buf.WriteString(p.Type)
						buf.WriteString(" | - |\n")
					}
					buf.WriteString("\n")
				}
			}
		}
	}

	os.WriteFile(path, []byte(buf.String()), 0644)
	fmt.Printf("Generated: %s\n", path)
}

func generateRemoteDoc(types []ServerType, path string) {
	buf := &strings.Builder{}
	buf.WriteString("# Remote API\n\n")
	buf.WriteString("> 本文档由 `gomelo doc` 自动生成，")
	buf.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	buf.WriteString("\n\n")

	buf.WriteString("## 目录\n\n")
	buf.WriteString("- [Handler](#handler)\n")
	buf.WriteString("- [Remote](#remote)\n")
	buf.WriteString("- [Cron](#cron)\n")
	buf.WriteString("- [Filter](#filter)\n\n")

	buf.WriteString("---\n\n")

	for _, st := range types {
		if len(st.Remotes) == 0 {
			continue
		}

		buf.WriteString("## ")
		buf.WriteString(st.Name)
		buf.WriteString("\n\n")

		for _, r := range st.Remotes {
			buf.WriteString("### ")
			buf.WriteString(r.TypeName)
			buf.WriteString("\n\n")

			buf.WriteString("| 路由 | 方法 | 描述 |\n")
			buf.WriteString("|------|------|------|\n")

			for _, m := range r.Methods {
				route := strings.ToLower(st.Name + "." + m.Name)
				desc := m.Doc
				if desc == "" {
					desc = "-"
				}
				buf.WriteString("| ")
				buf.WriteString(route)
				buf.WriteString(" | ")
				buf.WriteString(m.Name)
				buf.WriteString(" | ")
				buf.WriteString(desc)
				buf.WriteString(" |\n")
			}

			buf.WriteString("\n")

			for _, m := range r.Methods {
				route := strings.ToLower(st.Name + "." + m.Name)
				buf.WriteString("#### ")
				buf.WriteString(route)
				buf.WriteString("\n\n")

				if m.Doc != "" {
					buf.WriteString(m.Doc)
					buf.WriteString("\n\n")
				}

				if len(m.Params) > 1 {
					buf.WriteString("**参数**:\n\n")
					buf.WriteString("| 字段 | 类型 | 说明 |\n")
					buf.WriteString("|------|------|------|\n")
					for i, p := range m.Params {
						if i == 0 {
							continue
						}
						buf.WriteString("| ")
						buf.WriteString(p.Name)
						buf.WriteString(" | ")
						buf.WriteString(p.Type)
						buf.WriteString(" | - |\n")
					}
					buf.WriteString("\n")
				}

				if len(m.Results) > 0 {
					buf.WriteString("**返回值**:\n\n")
					buf.WriteString("| 类型 | 说明 |\n")
					buf.WriteString("|------|------|\n")
					for _, r := range m.Results {
						buf.WriteString("| ")
						buf.WriteString(r.Type)
						buf.WriteString(" | - |\n")
					}
					buf.WriteString("\n")
				}
			}
		}
	}

	os.WriteFile(path, []byte(buf.String()), 0644)
	fmt.Printf("Generated: %s\n", path)
}

func generateCronDoc(types []ServerType, path string) {
	buf := &strings.Builder{}
	buf.WriteString("# Cron API\n\n")
	buf.WriteString("> 本文档由 `gomelo doc` 自动生成，")
	buf.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	buf.WriteString("\n\n")

	buf.WriteString("## 目录\n\n")
	buf.WriteString("- [Handler](#handler)\n")
	buf.WriteString("- [Remote](#remote)\n")
	buf.WriteString("- [Cron](#cron)\n")
	buf.WriteString("- [Filter](#filter)\n\n")

	buf.WriteString("---\n\n")

	for _, st := range types {
		if len(st.Crons) == 0 {
			continue
		}

		buf.WriteString("## ")
		buf.WriteString(st.Name)
		buf.WriteString("\n\n")

		buf.WriteString("| ID | 类型.方法 | 描述 |\n")
		buf.WriteString("|----|-----------|------|\n")

		for _, c := range st.Crons {
			for _, m := range c.Methods {
				id := strings.ToLower(c.TypeName + "." + m.Name)
				desc := m.Doc
				if desc == "" {
					desc = "-"
				}
				buf.WriteString("| ")
				buf.WriteString(id)
				buf.WriteString(" | ")
				buf.WriteString(c.TypeName + "." + m.Name)
				buf.WriteString(" | ")
				buf.WriteString(desc)
				buf.WriteString(" |\n")
			}
		}

		buf.WriteString("\n")
	}

	os.WriteFile(path, []byte(buf.String()), 0644)
	fmt.Printf("Generated: %s\n", path)
}

func generateFilterDoc(types []ServerType, path string) {
	buf := &strings.Builder{}
	buf.WriteString("# Filter API\n\n")
	buf.WriteString("> 本文档由 `gomelo doc` 自动生成，")
	buf.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	buf.WriteString("\n\n")

	buf.WriteString("## 目录\n\n")
	buf.WriteString("- [Handler](#handler)\n")
	buf.WriteString("- [Remote](#remote)\n")
	buf.WriteString("- [Cron](#cron)\n")
	buf.WriteString("- [Filter](#filter)\n\n")

	buf.WriteString("---\n\n")

	for _, st := range types {
		if len(st.Filters) == 0 {
			continue
		}

		buf.WriteString("## ")
		buf.WriteString(st.Name)
		buf.WriteString("\n\n")

		buf.WriteString("| 类型 | Before | After | 说明 |\n")
		buf.WriteString("|------|--------|-------|------|\n")

		for _, f := range st.Filters {
			before := "✗"
			if f.Before {
				before = "✓"
			}
			after := "✗"
			if f.After {
				after = "✓"
			}
			buf.WriteString("| ")
			buf.WriteString(f.TypeName)
			buf.WriteString(" | ")
			buf.WriteString(before)
			buf.WriteString(" | ")
			buf.WriteString(after)
			buf.WriteString(" | - |\n")
		}

		buf.WriteString("\n")
	}

	os.WriteFile(path, []byte(buf.String()), 0644)
	fmt.Printf("Generated: %s\n", path)
}