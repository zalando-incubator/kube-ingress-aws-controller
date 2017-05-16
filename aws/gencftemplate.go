// +build ignore

// This program is run via "go generate" (via a directive in cf.go) to generate cftemplate.go.
//
// It creates cftemplate.go with a string constant with the content of the file ingress-cf-template.yaml. This constant
// is used to create the ALS stacks.
package main

import (
	"bytes"
	"go/parser"
	"log"

	"go/token"

	"fmt"
	"go/ast"
	"go/format"
	"io/ioutil"
)

var fset = token.NewFileSet()

const target = "cftemplate.go"

func main() {
	af, err := parser.ParseFile(fset, "cf.go", nil, parser.PackageClauseOnly)
	if err != nil {
		log.Fatalf("parser.ParseFile: %v", err)
	}

	buf, err := ioutil.ReadFile("ingress-cf-template.yaml")
	if err != nil {
		log.Fatal("ioutil.ReadFile: %v", err)
	}

	af.Comments = []*ast.CommentGroup{
		{},
	}
	var pkg bytes.Buffer
	if err := format.Node(&pkg, fset, af); err != nil {
		log.Fatalf("format.Node: %v", err)
	}

	var src bytes.Buffer
	fmt.Fprintln(&src, "// DO NOT EDIT; AUTO-GENERATED from cf.go using gencftemplate.go")
	src.Write(pkg.Bytes())
	src.WriteString("\nconst templateYAML = `")
	src.Write(buf)
	src.WriteString("`\n")

	// Final gofmt.
	out, err := format.Source(src.Bytes())
	if err != nil {
		log.Fatalf("format.Source: %v on\n%s", err, src)
	}

	if err := ioutil.WriteFile(target, out, 0644); err != nil {
		log.Fatal(err)
	}
}
