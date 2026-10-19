package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// This is a legacy composition contract, not persisted recovery runtime proof.
// A cold Space verifier must be able to obtain Voice's real public keys before
// the initial authority-dependent admission reconciliation starts.
func TestLegacyPublicJWKSStartsBeforeSpaceRecovery(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var publish, recover token.Pos
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "startSpaceMediaRuntime" {
			recover = call.Pos()
		}
		if method, ok := call.Fun.(*ast.SelectorExpr); ok {
			if receiver, ok := method.X.(*ast.Ident); ok && receiver.Name == "principaljwks" && method.Sel.Name == "Start" || method.Sel.Name == "ListenAndServeTLS" {
				publish = call.Pos()
			}
		}
		return true
	})
	if recover == 0 || publish == 0 || publish > recover {
		t.Fatal("legacy Voice must bind and start its real public TLS JWKS before initial Space admission recovery")
	}
}
