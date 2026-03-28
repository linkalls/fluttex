package compiler

import (
	"fmt"
	"path/filepath"
	"strings"
	"os"
    "os/exec"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/parser"
)

type Transpiler struct {
	source string
	opts   ast.SourceFileParseOptions
}

func NewTranspiler() *Transpiler {
    cwd, _ := os.Getwd()
	return &Transpiler{
		opts: ast.SourceFileParseOptions{
			FileName: filepath.Join(cwd, "temp.tsx"),
		},
	}
}

func (t *Transpiler) Transpile(source string) (string, error) {
	t.source = source

	file := parser.ParseSourceFile(t.opts, source, core.ScriptKindTSX)

	var sb strings.Builder

	sb.WriteString("import 'package:flutter/material.dart';\n\n")

	// Top level pass for imports and components
	file.AsNode().ForEachChild(func(node *ast.Node) bool {
		if node.Kind == ast.KindFunctionDeclaration {
			decl := t.visitFunctionDeclaration(node)
			if decl != "" {
				sb.WriteString(decl)
				sb.WriteString("\n\n")
			}
		}
		return false
	})

    // Format output via dart format
    tmpFile, err := os.CreateTemp("", "fluttex_*.dart")
    if err != nil {
        return sb.String(), nil
    }
    defer os.Remove(tmpFile.Name())
    tmpFile.WriteString(sb.String())
    tmpFile.Close()

    cmd := exec.Command("dart", "format", tmpFile.Name())
    if err := cmd.Run(); err == nil {
        formatted, err := os.ReadFile(tmpFile.Name())
        if err == nil {
            return string(formatted), nil
        }
    }

	return sb.String(), nil
}

func (t *Transpiler) visitFunctionDeclaration(node *ast.Node) string {
	name := "Unknown"
	var returnExpr *ast.Node
	isStateful := false
    stateVar := ""
    stateSetter := ""
    var initialVal string

	node.ForEachChild(func(child *ast.Node) bool {
		if child.Kind == ast.KindIdentifier {
			name = child.AsIdentifier().Text
		}
		if child.Kind == ast.KindBlock {
			child.ForEachChild(func(stmt *ast.Node) bool {
                if stmt.Kind == ast.KindVariableStatement {
                    stmt.ForEachChild(func(varDeclList *ast.Node) bool {
                        if varDeclList.Kind == ast.KindVariableDeclarationList {
                            varDeclList.ForEachChild(func(varDecl *ast.Node) bool {
                                if varDecl.Kind == ast.KindVariableDeclaration {
                                    isUseState := false
                                    varDecl.ForEachChild(func(vChild *ast.Node) bool {
                                        if vChild.Kind == ast.KindCallExpression {
                                            vChild.ForEachChild(func(callChild *ast.Node) bool {
                                                if callChild.Kind == ast.KindIdentifier && callChild.AsIdentifier().Text == "useState" {
                                                    isUseState = true
                                                }
                                                if isUseState && callChild.Kind == ast.KindNumericLiteral {
                                                    initialVal = callChild.AsNumericLiteral().Text
                                                }
                                                return false
                                            })
                                        }
                                        return false
                                    })

                                    if isUseState {
                                        isStateful = true
                                        varDecl.ForEachChild(func(vChild *ast.Node) bool {
                                            if vChild.Kind == ast.KindArrayBindingPattern {
                                                idx := 0
                                                vChild.ForEachChild(func(bindingElem *ast.Node) bool {
                                                    bindingElem.ForEachChild(func(id *ast.Node) bool {
                                                        if id.Kind == ast.KindIdentifier {
                                                            if idx == 0 {
                                                                stateVar = id.AsIdentifier().Text
                                                            } else if idx == 1 {
                                                                stateSetter = id.AsIdentifier().Text
                                                            }
                                                        }
                                                        return false
                                                    })
                                                    idx++
                                                    return false
                                                })
                                            }
                                            return false
                                        })
                                    }
                                }
                                return false
                            })
                        }
                        return false
                    })
                }
				if stmt.Kind == ast.KindReturnStatement {
					stmt.ForEachChild(func(expr *ast.Node) bool {
						returnExpr = expr
						return false
					})
				}
				return false
			})
		}
		return false
	})

	widgetCode := "Container()"
	if returnExpr != nil {
		widgetCode = t.visitJsxNode(returnExpr)
	}

	var sb strings.Builder

    if isStateful {
        sb.WriteString(fmt.Sprintf("class %s extends StatefulWidget {\n", name))
        sb.WriteString(fmt.Sprintf("  const %s({super.key});\n\n", name))
        sb.WriteString(fmt.Sprintf("  @override\n  State<%s> createState() => _%sState();\n}\n\n", name, name))

        sb.WriteString(fmt.Sprintf("class _%sState extends State<%s> {\n", name, name))
        sb.WriteString(fmt.Sprintf("  int %s = %s;\n\n", stateVar, initialVal)) // basic assumption of int for now

        // rudimentary replacement of state setters in widgetCode
        // This is a naive string replacement for the hackathon MVP logic constraints.
        // E.g. setCount(count + 1) -> setState(() { count = count + 1; })
        if stateSetter != "" && stateVar != "" {
            searchStr := fmt.Sprintf("%s(%s + 1)", stateSetter, stateVar)
            replaceStr := fmt.Sprintf("setState(() { %s = %s + 1; })", stateVar, stateVar)
            widgetCode = strings.ReplaceAll(widgetCode, searchStr, replaceStr)

            // replace plain stateSetter()
            searchStr2 := fmt.Sprintf("() => %s", stateSetter)
            replaceStr2 := fmt.Sprintf("() { setState(() { %s = ... }); }", stateVar) // placeholder
            widgetCode = strings.ReplaceAll(widgetCode, searchStr2, replaceStr2)
        }

        sb.WriteString("  @override\n")
        sb.WriteString("  Widget build(BuildContext context) {\n")
        sb.WriteString(fmt.Sprintf("    return %s;\n", widgetCode))
        sb.WriteString("  }\n")
        sb.WriteString("}")
    } else {
        sb.WriteString(fmt.Sprintf("class %s extends StatelessWidget {\n", name))
        sb.WriteString(fmt.Sprintf("  const %s({super.key});\n\n", name))
        sb.WriteString("  @override\n")
        sb.WriteString("  Widget build(BuildContext context) {\n")
        sb.WriteString(fmt.Sprintf("    return %s;\n", widgetCode))
        sb.WriteString("  }\n")
        sb.WriteString("}")
    }

	return sb.String()
}

func (t *Transpiler) visitJsxNode(node *ast.Node) string {
	if node.Kind == ast.KindParenthesizedExpression {
		var res string
		node.ForEachChild(func(child *ast.Node) bool {
			res = t.visitJsxNode(child)
			return false
		})
		return res
	}

	if node.Kind == ast.KindJsxElement {
		var opening *ast.Node
		var children []string

		node.ForEachChild(func(child *ast.Node) bool {
			if child.Kind == ast.KindJsxOpeningElement {
				opening = child
			} else if child.Kind == ast.KindJsxElement || child.Kind == ast.KindJsxSelfClosingElement {
				children = append(children, t.visitJsxNode(child))
			} else if child.Kind == ast.KindJsxText {
				text := child.AsJsxText().Text
				text = strings.TrimSpace(text)
				if text != "" {
					children = append(children, fmt.Sprintf("const Text('%s')", text))
				}
			} else if child.Kind == ast.KindJsxExpression {
				// We don't handle expressions inside JSX properly yet.
			}
			return false
		})

		tagName := ""
        props := make(map[string]string)
		if opening != nil {
			opening.ForEachChild(func(child *ast.Node) bool {
				if child.Kind == ast.KindIdentifier {
					tagName = child.AsIdentifier().Text
				}
                if child.Kind == ast.KindJsxAttributes {
                    child.ForEachChild(func(attr *ast.Node) bool {
                        if attr.Kind == ast.KindJsxAttribute {
                            attrName := ""
                            attrValue := ""
                            attr.ForEachChild(func(aChild *ast.Node) bool {
                                if aChild.Kind == ast.KindIdentifier {
                                    attrName = aChild.AsIdentifier().Text
                                } else if aChild.Kind == ast.KindStringLiteral {
                                    attrValue = fmt.Sprintf("'%s'", aChild.AsStringLiteral().Text)
                                } else if aChild.Kind == ast.KindJsxExpression {
                                    aChild.ForEachChild(func(expr *ast.Node) bool {
                                        if expr.Kind == ast.KindArrowFunction {
                                            // Hack for MVP inline setters
                                            exprStr := ""
                                            expr.ForEachChild(func(eChild *ast.Node) bool {
                                                if eChild.Kind == ast.KindCallExpression {
                                                    var caller string
                                                    var arg string
                                                    eChild.ForEachChild(func(cChild *ast.Node) bool {
                                                        if cChild.Kind == ast.KindIdentifier {
                                                            caller = cChild.AsIdentifier().Text
                                                        }
                                                        if cChild.Kind == ast.KindBinaryExpression {
                                                            var left, op, right string
                                                            cChild.ForEachChild(func(bChild *ast.Node) bool {
                                                                if bChild.Kind == ast.KindIdentifier {
                                                                    left = bChild.AsIdentifier().Text
                                                                } else if bChild.Kind == ast.KindPlusToken {
                                                                    op = "+"
                                                                } else if bChild.Kind == ast.KindNumericLiteral {
                                                                    right = bChild.AsNumericLiteral().Text
                                                                }
                                                                return false
                                                            })
                                                            arg = fmt.Sprintf("%s %s %s", left, op, right)
                                                        }
                                                        return false
                                                    })
                                                    if caller != "" && arg != "" {
                                                        exprStr = fmt.Sprintf("() { %s(%s); }", caller, arg)
                                                    }
                                                }
                                                return false
                                            })
                                            if exprStr == "" {
                                                if attrName == "onChange" && tagName == "input" {
                                                    exprStr = "(_) {}"
                                                } else {
                                                    exprStr = "() {}"
                                                }
                                            }
                                            attrValue = exprStr
                                        } else if expr.Kind == ast.KindIdentifier {
                                            attrValue = expr.AsIdentifier().Text
                                        }
                                        return false
                                    })
                                }
                                return false
                            })
                            props[attrName] = attrValue
                        }
                        return false
                    })
                }
				return false
			})
		}

		return t.mapJsxToFlutter(tagName, props, children)
	}

    if node.Kind == ast.KindJsxSelfClosingElement {
        tagName := ""
        props := make(map[string]string)
        node.ForEachChild(func(child *ast.Node) bool {
            if child.Kind == ast.KindIdentifier {
                tagName = child.AsIdentifier().Text
            }
            if child.Kind == ast.KindJsxAttributes {
                child.ForEachChild(func(attr *ast.Node) bool {
                    if attr.Kind == ast.KindJsxAttribute {
                        attrName := ""
                        attrValue := ""
                        attr.ForEachChild(func(aChild *ast.Node) bool {
                            if aChild.Kind == ast.KindIdentifier {
                                attrName = aChild.AsIdentifier().Text
                            } else if aChild.Kind == ast.KindStringLiteral {
                                attrValue = fmt.Sprintf("'%s'", aChild.AsStringLiteral().Text)
                            } else if aChild.Kind == ast.KindJsxExpression {
                                aChild.ForEachChild(func(expr *ast.Node) bool {
                                    if expr.Kind == ast.KindArrowFunction {
                                        if attrName == "onChange" && tagName == "input" {
                                            attrValue = "(_) {}"
                                        } else {
                                            attrValue = "() {}"
                                        }
                                    } else if expr.Kind == ast.KindIdentifier {
                                        attrValue = expr.AsIdentifier().Text
                                    }
                                    return false
                                })
                            }
                            return false
                        })
                        props[attrName] = attrValue
                    }
                    return false
                })
            }
            return false
        })
        return t.mapJsxToFlutter(tagName, props, nil)
    }

	return "Container()"
}

func (t *Transpiler) mapJsxToFlutter(tagName string, props map[string]string, children []string) string {
	childStr := ""
	if len(children) == 1 {
		childStr = fmt.Sprintf("child: %s", children[0])
	} else if len(children) > 1 {
		childStr = fmt.Sprintf("children: [\n%s\n]", strings.Join(children, ",\n"))
	}

    propsStr := ""
    if val, ok := props["data-testid"]; ok {
        propsStr += fmt.Sprintf("key: const Key(%s), ", val)
    }

    onPressedStr := "() {}"
    if val, ok := props["onClick"]; ok {
        onPressedStr = val
    }

    onChangedStr := ""
    if val, ok := props["onChange"]; ok {
        onChangedStr = fmt.Sprintf("onChanged: %s, ", val)
    }

	switch tagName {
	case "div", "View":
		if childStr == "" {
			return fmt.Sprintf("Container(%s)", propsStr)
		}
		if len(children) > 1 {
			return fmt.Sprintf("Column(%s%s)", propsStr, childStr)
		}
		return fmt.Sprintf("Container(%s%s)", propsStr, childStr)
	case "span", "Text":
		// Handled implicitly via JsxText mapping generally, but if explicitly wrapped:
		if len(children) == 1 && strings.HasPrefix(children[0], "const Text(") {
            if propsStr != "" {
                return fmt.Sprintf("Container(%schild: %s)", propsStr, children[0])
            }
			return children[0]
		}
		return fmt.Sprintf("Text(%s)", childStr)
	case "button":
        if childStr != "" {
		    return fmt.Sprintf("ElevatedButton(%sonPressed: %s, %s)", propsStr, onPressedStr, childStr)
        }
		return fmt.Sprintf("ElevatedButton(%sonPressed: %s)", propsStr, onPressedStr)
    case "input":
        return fmt.Sprintf("TextField(%s%s)", propsStr, onChangedStr)
    case "Icon":
        iconName := "Icons.check"
        if val, ok := props["name"]; ok {
            if val == "'check'" {
                iconName = "Icons.check"
            }
        }
        if propsStr != "" {
            return fmt.Sprintf("Container(%schild: const Icon(%s))", propsStr, iconName)
        }
        return fmt.Sprintf("const Icon(%s)", iconName)
	default:
		if childStr != "" {
			return fmt.Sprintf("Container(%s%s)", propsStr, childStr)
		}
		return fmt.Sprintf("Container(%s)", propsStr)
	}
}
