// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// gen-condition-docs generates markdown documentation for status conditions
// defined in the API types. This is needed because the per-CR condition type
// aliases (e.g. IstioConditionType = ConditionType) are transparent to
// crd-ref-docs, so their enum values don't appear in the generated API reference.
//
// Usage: go run ./hack/gen-condition-docs [api-dir]
// Output is written to stdout; typically appended to the API reference markdown.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type entry struct {
	value string // constant string value, e.g. "Reconciled"
	doc   string // doc comment
}

type condGroup struct {
	condType *entry  // the ConditionType in this const block, if any
	reasons  []entry // the ConditionReasons in this const block
}

func main() {
	apiDir := "api/v1"
	if len(os.Args) > 1 {
		apiDir = os.Args[1]
	}

	matches, err := filepath.Glob(filepath.Join(apiDir, "*_types.go"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error globbing %s: %v\n", apiDir, err)
		os.Exit(1)
	}
	sort.Strings(matches)

	fset := token.NewFileSet()
	crGroups := map[string][]condGroup{}

	for _, filePath := range matches {
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", filePath, err)
			os.Exit(1)
		}

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				continue
			}

			perCR := map[string]*condGroup{}

			for _, spec := range genDecl.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || vs.Type == nil {
					continue
				}

				typStr := identName(vs.Type)
				isType := strings.HasSuffix(typStr, "ConditionType")
				isReason := strings.HasSuffix(typStr, "ConditionReason")
				if !isType && !isReason {
					continue
				}

				cr := strings.TrimSuffix(strings.TrimSuffix(typStr, "ConditionType"), "ConditionReason")

				for i := range vs.Names {
					val := ""
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok {
							val = strings.Trim(lit.Value, `"`)
						}
					}
					doc := ""
					if vs.Doc != nil {
						doc = collapseWhitespace(vs.Doc.Text())
					}

					if perCR[cr] == nil {
						perCR[cr] = &condGroup{}
					}

					e := entry{value: val, doc: doc}
					if isType {
						perCR[cr].condType = &e
					} else {
						perCR[cr].reasons = append(perCR[cr].reasons, e)
					}
				}
			}

			for cr, g := range perCR {
				crGroups[cr] = append(crGroups[cr], *g)
			}
		}
	}

	// Render in a stable order
	order := []string{"Istio", "IstioRevision", "IstioRevisionTag", "IstioCNI", "ZTunnel"}
	for cr := range crGroups {
		if !contains(order, cr) {
			order = append(order, cr)
		}
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("## Conditions Reference")
	fmt.Println()
	fmt.Println("Each resource has a set of conditions in its status that indicate its current state.")
	fmt.Println("The `status` of each condition is one of `True`, `False`, or `Unknown`.")

	for _, cr := range order {
		groups, ok := crGroups[cr]
		if !ok {
			continue
		}

		fmt.Println()
		fmt.Printf("### %s\n", cr)

		for _, g := range groups {
			if g.condType != nil {
				fmt.Println()
				fmt.Printf("**`%s`** — %s\n", g.condType.value, g.condType.doc)
			}

			if len(g.reasons) > 0 {
				if g.condType == nil {
					fmt.Println()
					fmt.Println("*General reasons:*")
				}
				fmt.Println()
				fmt.Println("| Reason | Description |")
				fmt.Println("| --- | --- |")
				for _, r := range g.reasons {
					fmt.Printf("| `%s` | %s |\n", r.value, r.doc)
				}
			}
		}
	}
	fmt.Println()
}

func identName(expr ast.Expr) string {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func collapseWhitespace(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}
