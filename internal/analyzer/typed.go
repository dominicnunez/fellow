package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

const (
	loadPatternAll = "./..."
)

type declarationKey struct {
	Kind     string
	Name     string
	Receiver string
	Struct   string
	File     string
	Line     int
}

type typedDeclaration struct {
	key declarationKey
	obj types.Object
}

func applyTypedInfo(root string, module *moduleState, opts Options) {
	if len(module.packages) == 0 {
		return
	}

	pkgs, err := packages.Load(&packages.Config{
		Dir:   module.absDir,
		Tests: opts.IncludeTests,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes,
	}, loadPatternAll)
	if err != nil || len(pkgs) == 0 {
		return
	}

	typedDecls := make(map[declarationKey][]typedDeclaration)
	usesByObject := make(map[types.Object][]codeUse)
	for _, pkg := range pkgs {
		if pkg == nil || pkg.TypesInfo == nil || len(pkg.Syntax) == 0 {
			continue
		}
		collectTypedDeclarations(root, pkg, typedDecls)
		collectTypedUses(root, pkg, usesByObject)
	}
	interfaceMethodUses := collectInterfaceMethodUses(pkgs, typedDecls)

	markTypedDeclarations(module, typedDecls, usesByObject, interfaceMethodUses)
}

func collectTypedDeclarations(root string, pkg *packages.Package, typedDecls map[declarationKey][]typedDeclaration) {
	for _, file := range pkg.Syntax {
		collectTypedValueDeclarations(root, pkg, file, typedDecls)
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case nil:
				return true
			case *ast.FuncDecl:
				obj := pkg.TypesInfo.Defs[n.Name]
				if obj == nil {
					return false
				}
				kind := declarationFunction
				receiver := ""
				if n.Recv != nil {
					kind = declarationMethod
					receiver = receiverTypeName(n.Recv)
				}
				key := declarationKey{
					Kind:     kind,
					Name:     n.Name.Name,
					Receiver: receiver,
					File:     typedRelPath(root, pkg, n.Name.Pos()),
					Line:     pkg.Fset.Position(n.Name.Pos()).Line,
				}
				typedDecls[key] = append(typedDecls[key], typedDeclaration{key: key, obj: obj})
				return false
			case *ast.TypeSpec:
				obj := pkg.TypesInfo.Defs[n.Name]
				if obj == nil {
					return true
				}
				kind := declarationType
				switch n.Type.(type) {
				case *ast.StructType:
					kind = declarationStruct
				case *ast.InterfaceType:
					kind = declarationInterface
				}
				key := declarationKey{
					Kind: kind,
					Name: n.Name.Name,
					File: typedRelPath(root, pkg, n.Name.Pos()),
					Line: pkg.Fset.Position(n.Name.Pos()).Line,
				}
				typedDecls[key] = append(typedDecls[key], typedDeclaration{key: key, obj: obj})

				structType, ok := n.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {
						fieldObj := pkg.TypesInfo.Defs[name]
						if fieldObj == nil {
							continue
						}
						fieldKey := declarationKey{
							Kind:   declarationField,
							Name:   name.Name,
							Struct: n.Name.Name,
							File:   typedRelPath(root, pkg, name.Pos()),
							Line:   pkg.Fset.Position(name.Pos()).Line,
						}
						typedDecls[fieldKey] = append(typedDecls[fieldKey], typedDeclaration{key: fieldKey, obj: fieldObj})
					}
				}
				return true
			}

			return true
		})
	}
}

func collectTypedValueDeclarations(root string, pkg *packages.Package, file *ast.File, typedDecls map[declarationKey][]typedDeclaration) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || (genDecl.Tok != token.VAR && genDecl.Tok != token.CONST) {
			continue
		}

		kind := declarationVar
		if genDecl.Tok == token.CONST {
			kind = declarationConst
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range valueSpec.Names {
				obj := pkg.TypesInfo.Defs[name]
				if obj == nil {
					continue
				}
				key := declarationKey{
					Kind: kind,
					Name: name.Name,
					File: typedRelPath(root, pkg, name.Pos()),
					Line: pkg.Fset.Position(name.Pos()).Line,
				}
				typedDecls[key] = append(typedDecls[key], typedDeclaration{key: key, obj: obj})
			}
		}
	}
}

func collectTypedUses(root string, pkg *packages.Package, usesByObject map[types.Object][]codeUse) {
	for ident, obj := range pkg.TypesInfo.Uses {
		if obj == nil {
			continue
		}
		usesByObject[obj] = append(usesByObject[obj], typedUseAt(root, pkg, ident.Pos()))
	}

	for selector, selection := range pkg.TypesInfo.Selections {
		if selection == nil || selection.Obj() == nil {
			continue
		}
		usesByObject[selection.Obj()] = append(usesByObject[selection.Obj()], typedUseAt(root, pkg, selector.Sel.Pos()))
	}
}

func markTypedDeclarations(module *moduleState, typedDecls map[declarationKey][]typedDeclaration, usesByObject map[types.Object][]codeUse, interfaceMethodUses map[types.Object]bool) {
	for pkgIndex := range module.packages {
		for declIndex := range module.packages[pkgIndex].declarations {
			decl := &module.packages[pkgIndex].declarations[declIndex]
			matches := typedDecls[keyForDeclaration(*decl)]
			if len(matches) == 0 {
				continue
			}

			decl.Typed = true
			for _, match := range matches {
				if interfaceMethodUses[match.obj] || hasUseOutside(usesByObject[match.obj], *decl) {
					decl.TypedUsed = true
					break
				}
			}
		}
	}
}

func collectInterfaceMethodUses(pkgs []*packages.Package, typedDecls map[declarationKey][]typedDeclaration) map[types.Object]bool {
	interfaces := collectInterfaceTypes(pkgs)
	if len(interfaces) == 0 {
		return nil
	}

	used := make(map[types.Object]bool)
	for _, declarations := range typedDecls {
		for _, decl := range declarations {
			method, ok := decl.obj.(*types.Func)
			if !ok || !methodHasReceiver(method) {
				continue
			}
			for _, iface := range interfaces {
				if !interfaceHasMethod(iface, method.Name()) {
					continue
				}
				if receiverImplementsInterface(method, iface) {
					used[method] = true
					break
				}
			}
		}
	}

	return used
}

func collectInterfaceTypes(pkgs []*packages.Package) []*types.Interface {
	var interfaces []*types.Interface
	seenTypes := make(map[types.Type]struct{})
	seenInterfaces := make(map[*types.Interface]struct{})

	var addType func(types.Type)
	addType = func(typ types.Type) {
		if typ == nil {
			return
		}
		if _, ok := seenTypes[typ]; ok {
			return
		}
		seenTypes[typ] = struct{}{}

		switch t := typ.(type) {
		case *types.Named:
			addType(t.Underlying())
		case *types.Pointer:
			addType(t.Elem())
		case *types.Slice:
			addType(t.Elem())
		case *types.Array:
			addType(t.Elem())
		case *types.Map:
			addType(t.Key())
			addType(t.Elem())
		case *types.Chan:
			addType(t.Elem())
		case *types.Signature:
			if t.Recv() != nil {
				addType(t.Recv().Type())
			}
			addTupleTypes(t.Params(), addType)
			addTupleTypes(t.Results(), addType)
		case *types.Struct:
			for i := range t.NumFields() {
				addType(t.Field(i).Type())
			}
		case *types.Interface:
			t.Complete()
			if _, ok := seenInterfaces[t]; !ok {
				seenInterfaces[t] = struct{}{}
				interfaces = append(interfaces, t)
			}
			for i := range t.NumEmbeddeds() {
				addType(t.EmbeddedType(i))
			}
		}
	}

	for _, pkg := range pkgs {
		if pkg == nil || pkg.TypesInfo == nil {
			continue
		}
		for _, obj := range pkg.TypesInfo.Defs {
			if obj != nil {
				addType(obj.Type())
			}
		}
		for _, obj := range pkg.TypesInfo.Uses {
			if obj != nil {
				addType(obj.Type())
			}
		}
		for _, value := range pkg.TypesInfo.Types {
			addType(value.Type)
		}
	}

	return interfaces
}

func addTupleTypes(tuple *types.Tuple, addType func(types.Type)) {
	if tuple == nil {
		return
	}
	for i := range tuple.Len() {
		addType(tuple.At(i).Type())
	}
}

func methodHasReceiver(method *types.Func) bool {
	sig, ok := method.Type().(*types.Signature)
	return ok && sig.Recv() != nil
}

func interfaceHasMethod(iface *types.Interface, name string) bool {
	iface.Complete()
	for i := range iface.NumMethods() {
		if iface.Method(i).Name() == name {
			return true
		}
	}

	return false
}

func receiverImplementsInterface(method *types.Func, iface *types.Interface) bool {
	sig, ok := method.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}

	receiver := sig.Recv().Type()
	if types.Implements(receiver, iface) {
		return true
	}
	if _, ok := receiver.(*types.Pointer); ok {
		return false
	}

	return types.Implements(types.NewPointer(receiver), iface)
}

func keyForDeclaration(decl declaration) declarationKey {
	return declarationKey{
		Kind:     decl.Kind,
		Name:     decl.Name,
		Receiver: decl.Receiver,
		Struct:   decl.Struct,
		File:     decl.File,
		Line:     decl.Line,
	}
}

func typedUseAt(root string, pkg *packages.Package, pos token.Pos) codeUse {
	return codeUse{
		File: typedRelPath(root, pkg, pos),
		Line: pkg.Fset.Position(pos).Line,
	}
}

func typedRelPath(root string, pkg *packages.Package, pos token.Pos) string {
	filename := pkg.Fset.Position(pos).Filename
	if filename == "" || strings.HasPrefix(filename, "<") {
		return filename
	}

	return relPath(root, filename)
}
