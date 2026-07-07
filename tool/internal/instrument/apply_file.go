// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrument

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.opentelemetry.io/otelc/tool/ex"
	"go.opentelemetry.io/otelc/tool/internal/ast"
	"go.opentelemetry.io/otelc/tool/internal/rule"
	"go.opentelemetry.io/otelc/tool/util"
)

func stripBuildIgnoreTag(content string) string {
	return strings.ReplaceAll(content, "//go:build ignore", "")
}

// packageNameFromCompileArgs reads the package name from the first parsable Go
// source file among the compile arguments. Used when setup-time resolution
// could not name the package (synthetic test mains, cover-rewritten sources).
func (ip *InstrumentPhase) packageNameFromCompileArgs() (string, error) {
	for _, arg := range ip.compileArgs {
		if !strings.HasSuffix(arg, ".go") || !util.PathExists(arg) {
			continue
		}
		name, err := ast.ParsePackageName(arg)
		if err == nil && name != "" {
			return name, nil
		}
	}
	return "", ex.Newf("no parsable Go source file among compile arguments")
}

// applyFileRule introduces the new file to the target package at compile time.
func (ip *InstrumentPhase) applyFileRule(ctx context.Context, rule *rule.InstFileRule, pkgName string) error {
	// List all files in the rule module path
	files, err := util.ListFiles(rule.ResolvedPath)
	if err != nil {
		return ex.Wrapf(err, "listing files for rule %s in dir %s (import path %s)",
			rule.Name, rule.ResolvedPath, rule.Path)
	}

	// Find the new file we want to introduce
	index := slices.IndexFunc(files, func(file string) bool {
		return strings.HasSuffix(file, rule.File)
	})
	if index == -1 {
		return ex.Newf("file %s not found", rule.File)
	}
	file := files[index]

	// Parse the new file into AST nodes and modify it as needed.
	// Keep processing in-memory to avoid mutating shared temp rule files.
	data, err := os.ReadFile(file)
	if err != nil {
		return ex.Wrapf(err, "reading rule source file %s", file)
	}
	root, err := ast.NewAstParser().ParseSource(stripBuildIgnoreTag(string(data)))
	if err != nil {
		return ex.Wrapf(err, "parsing rule source file %s", file)
	}
	// Always rename the package name to the target package name.
	//
	// The setup-time name (pkgName) is empty for packages that cmd/go
	// synthesizes after setup ran — the `go test` main package and
	// cover-rewritten mains — and writing an empty name produces an
	// unparsable file ("package \n"). The files being compiled are the
	// authoritative source of the package name at this point, so read it
	// from them when setup could not resolve one.
	if pkgName == "" {
		pkgName, err = ip.packageNameFromCompileArgs()
		if err != nil {
			return ex.Wrapf(err, "resolving package name for file rule %s", rule.Name)
		}
	}
	root.Name.Name = pkgName

	// The file being added has its own imports that need to be in importcfg.
	// Without this, the compiler will fail with "could not import X" errors.
	if err = ip.updateImportConfigForFile(ctx, root, rule.Name); err != nil {
		return err
	}

	// Write back the modified AST to a new file in the working directory
	base := filepath.Base(rule.File)
	ext := filepath.Ext(base)
	newName := strings.TrimSuffix(base, ext)
	newFile := filepath.Join(ip.workDir, fmt.Sprintf("otelc.%s.go", newName))
	err = ast.WriteFile(newFile, root)
	if err != nil {
		return ex.Wrapf(err, "writing instrumented file %s", newFile)
	}
	ip.Info("Apply file rule", "rule", rule)

	// Add the new file as part of the source files to be compiled
	ip.addCompileArg(newFile)
	ip.keepForDebug(newFile)
	return nil
}
