package parser

import (
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// TestBasicTreeSitterParsing tests basic tree-sitter functionality without queries
func TestBasicTreeSitterParsing(t *testing.T) {
	// Simple Go code
	code := []byte("package main\n\nfunc Hello() string {\n    return \"hello\"\n}")

	// Create language
	language := sitter.NewLanguage(tree_sitter_go.Language())
	if language == nil {
		t.Fatal("Failed to create Go language")
	}

	// Create parser
	parser := sitter.NewParser()
	defer parser.Close()

	// Set language
	err := parser.SetLanguage(language)
	if err != nil {
		t.Fatalf("Failed to set language: %v", err)
	}

	// Parse
	tree := parser.Parse(code, nil)
	if tree == nil {
		t.Fatal("Failed to parse code")
	}
	defer tree.Close()

	// Check that we have a valid tree
	rootNode := tree.RootNode()
	if rootNode == nil {
		t.Fatal("No root node")
	}

	// Check that the root node has children (package declaration, function, etc.)
	if rootNode.ChildCount() == 0 {
		t.Fatal("Root node has no children")
	}

	t.Logf("Successfully parsed Go code. Root node has %d children", rootNode.ChildCount())
	t.Logf("Root node type: %s", rootNode.Kind())

	// Walk the tree to find function nodes
	var walkNodes func(*sitter.Node, int)
	walkNodes = func(node *sitter.Node, depth int) {
		if node == nil {
			return
		}

		nodeType := node.Kind()
		t.Logf("Depth %d: %s", depth, nodeType)

		if nodeType == "function_declaration" {
			t.Logf("Found function declaration at depth %d", depth)
		}

		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil {
				walkNodes(child, depth+1)
			}
		}
	}

	walkNodes(rootNode, 0)
}