package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

func TestGRPCReadinessWaitsUseIndependentTimeoutContexts(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), "main.go", source, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	timeoutContexts := make(map[*ast.Object]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) == 0 || len(assign.Rhs) == 0 || !isGRPCDialTimeoutContext(assign.Rhs[0]) {
			return true
		}
		if ctx, ok := assign.Lhs[0].(*ast.Ident); ok && ctx.Obj != nil {
			timeoutContexts[ctx.Obj] = true
		}
		return true
	})

	uses := make(map[*ast.Object]int)
	readinessWaits := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isWaitForGRPCReady(call) || len(call.Args) == 0 {
			return true
		}
		readinessWaits++
		ctx, ok := call.Args[0].(*ast.Ident)
		if !ok || ctx.Obj == nil || !timeoutContexts[ctx.Obj] {
			t.Errorf("waitForGRPCReady must receive a context created with context.WithTimeout(..., grpcclient.DialTimeoutFromEnv())")
			return true
		}
		uses[ctx.Obj]++
		return true
	})
	if readinessWaits != 5 {
		t.Errorf("expected readiness waits for Chat, Voice, User, Social, and Space; got %d", readinessWaits)
	}

	for _, count := range uses {
		if count > 1 {
			t.Errorf("a GRPC_DIAL_TIMEOUT context is shared by %d readiness waits; each dependency needs its own timeout", count)
		}
	}
}

func isGRPCDialTimeoutContext(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 2 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "WithTimeout" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "context" && isGRPCDialTimeout(call.Args[1])
}

func isGRPCDialTimeout(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "DialTimeoutFromEnv" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "grpcclient"
}

func isWaitForGRPCReady(call *ast.CallExpr) bool {
	function, ok := call.Fun.(*ast.Ident)
	return ok && function.Name == "waitForGRPCReady"
}
