package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Inspect the real composition, rather than a test-only feature switch: absence
// of the new relay from this root applies to every ordinary/Space/NATS config.
func TestProductionRootKeepsUnqualifiedRosterRelayDormant(t *testing.T) {
	root, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	compatibility := map[string]bool{"admissionRelay": false, "spaceMediaCoordinator": false}
	ast.Inspect(root, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && id.Name == "runRosterOutbox" {
			t.Error("production root must not activate the A3 roster relay before its canonical broker/consumer qualification")
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || method.Sel.Name != "Run" {
			return true
		}
		if receiver, ok := method.X.(*ast.Ident); ok {
			if _, required := compatibility[receiver.Name]; required {
				compatibility[receiver.Name] = true
			}
		}
		return true
	})
	for name, present := range compatibility {
		if !present {
			t.Errorf("existing compatibility/restrictive runtime %s.Run must remain wired", name)
		}
	}
	_ = runRosterOutbox
}
