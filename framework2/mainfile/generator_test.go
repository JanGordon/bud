package mainfile_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/matryer/is"
	"gitlab.com/mnm/bud/framework2/mainfile"
	"gitlab.com/mnm/bud/internal/imports"
)

func parse(code []byte) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, "", code, parser.DeclarationErrors)
}

func TestEmpty(t *testing.T) {
	is := is.New(t)
	code, err := mainfile.Generate(&mainfile.State{})
	is.NoErr(err)
	file, err := parse(code)
	is.NoErr(err)
	is.Equal(file.Name.Name, "main")
}

func TestImports(t *testing.T) {
	is := is.New(t)
	imports := imports.New()
	imports.AddStd("os", "context")
	imports.AddNamed("program", "app.com/.cli/program")
	code, err := mainfile.Generate(&mainfile.State{
		Imports: imports.List(),
	})
	is.NoErr(err)
	file, err := parse(code)
	is.NoErr(err)
	is.Equal(file.Name.Name, "main")
}