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
	compatibility := map[string]bool{"admissionRelay": false, "spaceMediaCoordinator": false}
	factoryCalled := false
	for _, path := range []string{"main.go", "space_media_runtime.go"} {
		root, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(root, func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok && id.Name == "runRosterOutbox" {
				t.Error("production root must not activate the A3 roster relay before its canonical broker/consumer qualification")
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && path == "main.go" && id.Name == "startSpaceMediaRuntime" {
				factoryCalled = true
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
	}
	if !factoryCalled {
		t.Error("production root must call the tested Space/P3 factory")
	}
	for name, present := range compatibility {
		if !present {
			t.Errorf("existing compatibility/restrictive runtime %s.Run must remain wired", name)
		}
	}
	// Keep the isolated implementation type-checked without making it a
	// production startup path. Its PG delivery tests remain independently useful.
	_ = runRosterOutbox
}
