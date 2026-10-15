package cleaner

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// AuditNoShellInjection audits a directory tree for shell injection patterns (POL-04, D-16).
// It verifies that external commands are invoked with tokenized argument slices and never
// via shell interpreters ("sh", "bash", "-c") or shell string interpolation.
func AuditNoShellInjection(rootDir string) ([]string, error) {
	var violations []string
	fset := token.NewFileSet()

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}

		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			fnName := getCallName(call)
			// Check calls to exec.Command, exec.CommandContext, Output, execCommand
			isExecCall := fnName == "exec.Command" ||
				fnName == "exec.CommandContext" ||
				strings.HasSuffix(fnName, ".Output") ||
				fnName == "execCommand"

			if !isExecCall {
				return true
			}

			// Determine command name argument index
			cmdArgIdx := 0
			if fnName == "exec.CommandContext" || strings.HasSuffix(fnName, ".Output") || fnName == "execCommand" {
				cmdArgIdx = 1
			}

			if len(call.Args) > cmdArgIdx {
				cmdArg := getLiteralString(call.Args[cmdArgIdx])
				if isShellBinary(cmdArg) {
					pos := fset.Position(call.Pos())
					violations = append(violations, fmt.Sprintf("%s:%d: invocation of shell binary %q in %s", path, pos.Line, cmdArg, fnName))
				}
			}

			// Check all arguments for "-c" flag
			for i := cmdArgIdx + 1; i < len(call.Args); i++ {
				argVal := getLiteralString(call.Args[i])
				if argVal == "-c" {
					pos := fset.Position(call.Args[i].Pos())
					violations = append(violations, fmt.Sprintf("%s:%d: forbidden '-c' flag passed in %s", path, pos.Line, fnName))
				}
			}

			return true
		})

		return nil
	})

	return violations, err
}

func getCallName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			return x.Name + "." + fn.Sel.Name
		}
		return "." + fn.Sel.Name
	default:
		return ""
	}
}

func getLiteralString(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.Trim(lit.Value, "\"`")
	}
	return ""
}

func isShellBinary(name string) bool {
	switch name {
	case "sh", "/bin/sh", "/usr/bin/sh", "bash", "/bin/bash", "/usr/bin/bash", "zsh", "/bin/zsh":
		return true
	default:
		return false
	}
}
