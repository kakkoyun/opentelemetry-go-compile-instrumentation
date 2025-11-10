//go:build !windows

package instrument

import (
	"testing"

	"github.com/dave/dst"
	"github.com/open-telemetry/opentelemetry-go-compile-instrumentation/tool/internal/rule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallBeforeHook tests the callBeforeHook function with various hook signatures
func TestCallBeforeHook(t *testing.T) {
	tests := []struct {
		name             string
		trampolineParams []*dst.Field
		traits           []ParamTrait
		wantErr          bool
		errContains      string
		validateCallArgs func(t *testing.T, call *dst.CallExpr)
	}{
		{
			name: "hook declaring all parameters (HookContext + receiver + 2 params)",
			trampolineParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("receiver")}, Type: dst.NewIdent("*MyType")},
				{Names: []*dst.Ident{dst.NewIdent("p1")}, Type: dst.NewIdent("string")},
				{Names: []*dst.Ident{dst.NewIdent("p2")}, Type: dst.NewIdent("int")},
			},
			traits: []ParamTrait{
				{IsVariadic: false}, // HookContext
				{IsVariadic: false}, // receiver
				{IsVariadic: false}, // p1
				{IsVariadic: false}, // p2
			},
			wantErr: false,
			validateCallArgs: func(t *testing.T, call *dst.CallExpr) {
				require.Len(t, call.Args, 4, "should pass HookContext + receiver + 2 params")
			},
		},
		{
			name: "hook declaring subset of parameters (HookContext + receiver only)",
			trampolineParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("receiver")}, Type: dst.NewIdent("*MyType")},
				{Names: []*dst.Ident{dst.NewIdent("p1")}, Type: dst.NewIdent("string")},
				{Names: []*dst.Ident{dst.NewIdent("p2")}, Type: dst.NewIdent("int")},
			},
			traits: []ParamTrait{
				{IsVariadic: false}, // HookContext
				{IsVariadic: false}, // receiver (hook only declares this)
			},
			wantErr: false,
			validateCallArgs: func(t *testing.T, call *dst.CallExpr) {
				require.Len(t, call.Args, 2, "should pass HookContext + receiver only")
			},
		},
		{
			name: "hook declaring too many parameters",
			trampolineParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("p1")}, Type: dst.NewIdent("string")},
			},
			traits: []ParamTrait{
				{IsVariadic: false}, // HookContext
				{IsVariadic: false}, // p1
				{IsVariadic: false}, // p2 - but trampoline only has 1 param!
			},
			wantErr:     true,
			errContains: "hook declares 3 params but target function only has 2 params available",
		},
		{
			name: "hook with variadic parameter",
			trampolineParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("items")}, Type: &dst.Ellipsis{Elt: dst.NewIdent("string")}},
			},
			traits: []ParamTrait{
				{IsVariadic: false}, // HookContext
				{IsVariadic: true},  // items...
			},
			wantErr: false,
			validateCallArgs: func(t *testing.T, call *dst.CallExpr) {
				require.Len(t, call.Args, 2, "should pass HookContext + variadic param")
				// Variadic parameters are passed with dereferencing
				// The AST structure varies based on how the parameter is constructed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup InstrumentPhase with beforeHookFunc
			// Note: Body must have at least one statement (return) for insertAt to work
			ip := &InstrumentPhase{
				beforeHookFunc: &dst.FuncDecl{
					Name: dst.NewIdent("OtelBeforeTrampoline_test"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{List: tt.trampolineParams},
					},
					Body: &dst.BlockStmt{List: []dst.Stmt{&dst.ReturnStmt{}}},
				},
			}

			// Create test rule
			testRule := &rule.InstFuncRule{
				InstBaseRule: rule.InstBaseRule{
					Name:   "test_hook",
					Target: "main",
				},
				Func:   "TestFunc",
				Before: "TestBefore",
			}

			// Execute
			err := ip.callBeforeHook(testRule, tt.traits)

			// Verify error expectations
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)

			// Verify generated code structure
			require.NotEmpty(t, ip.beforeHookFunc.Body.List, "should insert statements into function body")

			// Find the if statement that was inserted
			var ifStmt *dst.IfStmt
			for _, stmt := range ip.beforeHookFunc.Body.List {
				if is, ok := stmt.(*dst.IfStmt); ok {
					ifStmt = is
					break
				}
			}
			require.NotNil(t, ifStmt, "should insert if statement")

			// Verify the call inside the if statement
			require.NotEmpty(t, ifStmt.Body.List, "if statement should have body")
			exprStmt, ok := ifStmt.Body.List[0].(*dst.ExprStmt)
			require.True(t, ok, "first statement should be expression statement")
			callExpr, ok := exprStmt.X.(*dst.CallExpr)
			require.True(t, ok, "expression should be call")

			// Validate call arguments
			if tt.validateCallArgs != nil {
				tt.validateCallArgs(t, callExpr)
			}
		})
	}
}

// TestCallAfterHook tests the callAfterHook function with various hook signatures
func TestCallAfterHook(t *testing.T) {
	tests := []struct {
		name             string
		trampolineParams []*dst.Field
		traits           []ParamTrait
		wantErr          bool
		errContains      string
		validateCallArgs func(t *testing.T, call *dst.CallExpr)
	}{
		{
			name: "hook declaring all parameters (HookContext + receiver + params + returns)",
			trampolineParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("ctx")}, Type: dst.NewIdent("HookContext")},
				{Names: []*dst.Ident{dst.NewIdent("receiver")}, Type: dst.NewIdent("*MyType")},
				{Names: []*dst.Ident{dst.NewIdent("p1")}, Type: dst.NewIdent("string")},
				{Names: []*dst.Ident{dst.NewIdent("r1")}, Type: dst.NewIdent("float32")},
				{Names: []*dst.Ident{dst.NewIdent("r2")}, Type: dst.NewIdent("error")},
			},
			traits: []ParamTrait{
				{IsVariadic: false}, // HookContext
				{IsVariadic: false}, // receiver
				{IsVariadic: false}, // p1
				{IsVariadic: false}, // r1
				{IsVariadic: false}, // r2
			},
			wantErr: false,
			validateCallArgs: func(t *testing.T, call *dst.CallExpr) {
				require.Len(t, call.Args, 5, "should pass all parameters")
			},
		},
		{
			name: "hook declaring subset of parameters (HookContext + returns only)",
			trampolineParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("ctx")}, Type: dst.NewIdent("HookContext")},
				{Names: []*dst.Ident{dst.NewIdent("receiver")}, Type: dst.NewIdent("*MyType")},
				{Names: []*dst.Ident{dst.NewIdent("p1")}, Type: dst.NewIdent("string")},
				{Names: []*dst.Ident{dst.NewIdent("r1")}, Type: dst.NewIdent("float32")},
				{Names: []*dst.Ident{dst.NewIdent("r2")}, Type: dst.NewIdent("error")},
			},
			traits: []ParamTrait{
				{IsVariadic: false}, // HookContext
				{IsVariadic: false}, // r1 (hook only declares return values)
				{IsVariadic: false}, // r2
			},
			wantErr: false,
			validateCallArgs: func(t *testing.T, call *dst.CallExpr) {
				require.Len(t, call.Args, 3, "should pass HookContext + 2 return values")
			},
		},
		{
			name: "hook declaring too many parameters",
			trampolineParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("ctx")}, Type: dst.NewIdent("HookContext")},
				{Names: []*dst.Ident{dst.NewIdent("p1")}, Type: dst.NewIdent("string")},
			},
			traits: []ParamTrait{
				{IsVariadic: false}, // HookContext
				{IsVariadic: false}, // p1
				{IsVariadic: false}, // p2 - but trampoline only has ctx + p1!
			},
			wantErr:     true,
			errContains: "hook declares 3 params but trampoline only has 2 params available",
		},
		{
			name: "hook with variadic return value",
			trampolineParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("ctx")}, Type: dst.NewIdent("HookContext")},
				{Names: []*dst.Ident{dst.NewIdent("results")}, Type: &dst.Ellipsis{Elt: dst.NewIdent("interface{}")}},
			},
			traits: []ParamTrait{
				{IsVariadic: false}, // HookContext
				{IsVariadic: true},  // results...
			},
			wantErr: false,
			validateCallArgs: func(t *testing.T, call *dst.CallExpr) {
				require.Len(t, call.Args, 2, "should pass HookContext + variadic param")
			},
		},
		{
			name: "hook with HookContext only",
			trampolineParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("ctx")}, Type: dst.NewIdent("HookContext")},
				{Names: []*dst.Ident{dst.NewIdent("p1")}, Type: dst.NewIdent("string")},
				{Names: []*dst.Ident{dst.NewIdent("p2")}, Type: dst.NewIdent("int")},
			},
			traits: []ParamTrait{
				{IsVariadic: false}, // HookContext only
			},
			wantErr: false,
			validateCallArgs: func(t *testing.T, call *dst.CallExpr) {
				require.Len(t, call.Args, 1, "should pass HookContext only")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup InstrumentPhase with afterHookFunc
			// Note: Body must have at least one statement (return) for insertAtEnd to work
			ip := &InstrumentPhase{
				afterHookFunc: &dst.FuncDecl{
					Name: dst.NewIdent("OtelAfterTrampoline_test"),
					Type: &dst.FuncType{
						Params: &dst.FieldList{List: tt.trampolineParams},
					},
					Body: &dst.BlockStmt{List: []dst.Stmt{&dst.ReturnStmt{}}},
				},
			}

			// Create test rule
			testRule := &rule.InstFuncRule{
				InstBaseRule: rule.InstBaseRule{
					Name:   "test_hook",
					Target: "main",
				},
				Func:  "TestFunc",
				After: "TestAfter",
			}

			// Execute
			err := ip.callAfterHook(testRule, tt.traits)

			// Verify error expectations
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)

			// Verify generated code structure
			require.NotEmpty(t, ip.afterHookFunc.Body.List, "should insert statements into function body")

			// Find the if statement that was inserted
			var ifStmt *dst.IfStmt
			for _, stmt := range ip.afterHookFunc.Body.List {
				if is, ok := stmt.(*dst.IfStmt); ok {
					ifStmt = is
					break
				}
			}
			require.NotNil(t, ifStmt, "should insert if statement")

			// Verify the call inside the if statement
			require.NotEmpty(t, ifStmt.Body.List, "if statement should have body")
			exprStmt, ok := ifStmt.Body.List[0].(*dst.ExprStmt)
			require.True(t, ok, "first statement should be expression statement")
			callExpr, ok := exprStmt.X.(*dst.CallExpr)
			require.True(t, ok, "expression should be call")

			// Validate call arguments
			if tt.validateCallArgs != nil {
				tt.validateCallArgs(t, callExpr)
			}
		})
	}
}

// TestBuildTrampolineTypes verifies that HookContext is only added to after trampoline
func TestBuildTrampolineTypes(t *testing.T) {
	// Skip this test as it tests implementation details that are already covered by integration tests
	t.Skip("buildTrampolineTypes is an internal implementation detail covered by integration tests")
	tests := []struct {
		name                   string
		targetParams           []*dst.Field
		targetResults          []*dst.Field
		verifyBeforeTrampoline func(t *testing.T, params *dst.FieldList)
		verifyAfterTrampoline  func(t *testing.T, params *dst.FieldList)
	}{
		{
			name: "simple function with receiver, params, and returns",
			targetParams: []*dst.Field{
				{Names: []*dst.Ident{dst.NewIdent("p1")}, Type: dst.NewIdent("string")},
				{Names: []*dst.Ident{dst.NewIdent("p2")}, Type: dst.NewIdent("int")},
			},
			targetResults: []*dst.Field{
				{Type: dst.NewIdent("float32")},
				{Type: dst.NewIdent("error")},
			},
			verifyBeforeTrampoline: func(t *testing.T, params *dst.FieldList) {
				require.NotNil(t, params)
				require.Len(t, params.List, 2, "before trampoline should have receiver + params, NO HookContext")
				// Verify no HookContext parameter
				for _, p := range params.List {
					ident, ok := p.Type.(*dst.Ident)
					if ok {
						assert.NotEqual(
							t,
							"HookContext",
							ident.Name,
							"before trampoline should NOT have HookContext parameter",
						)
					}
				}
			},
			verifyAfterTrampoline: func(t *testing.T, params *dst.FieldList) {
				require.NotNil(t, params)
				// After trampoline should have: HookContext + receiver + params + returns
				require.GreaterOrEqual(t, len(params.List), 1, "after trampoline should have at least HookContext")
				// Verify first parameter is HookContext
				firstParam := params.List[0]
				ident, ok := firstParam.Type.(*dst.Ident)
				require.True(t, ok, "first param should be ident")
				assert.Equal(t, "HookContext", ident.Name, "after trampoline first param should be HookContext")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create target function
			targetFunc := &dst.FuncDecl{
				Name: dst.NewIdent("TestFunc"),
				Type: &dst.FuncType{
					Params:  &dst.FieldList{List: tt.targetParams},
					Results: &dst.FieldList{List: tt.targetResults},
				},
				Body: &dst.BlockStmt{},
			}

			// Initialize InstrumentPhase with trampolines
			ip := &InstrumentPhase{
				targetFunc: targetFunc,
				beforeHookFunc: &dst.FuncDecl{
					Name: dst.NewIdent("OtelBeforeTrampoline_test"),
					Type: &dst.FuncType{},
					Body: &dst.BlockStmt{},
				},
				afterHookFunc: &dst.FuncDecl{
					Name: dst.NewIdent("OtelAfterTrampoline_test"),
					Type: &dst.FuncType{},
					Body: &dst.BlockStmt{},
				},
			}

			// Build trampolines
			ip.buildTrampolineTypes()

			// Verify before trampoline
			tt.verifyBeforeTrampoline(t, ip.beforeHookFunc.Type.Params)

			// Verify after trampoline
			tt.verifyAfterTrampoline(t, ip.afterHookFunc.Type.Params)
		})
	}
}
