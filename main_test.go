package main

import (
	"os"
	"testing"
)

// Simple test tscn file content
const testTscnContent = `[gd_scene load_steps=2 format=3]

[node name="Root" type="Node2D"]

[node name="Child1" type="Control" parent="."]

[node name="Child2" type="Control" parent="."]

[node name="GrandChild" type="Button" parent="Child1"]
text = "Test Button"

[node name="DeepChild" type="Label" parent="Child1/GrandChild"]
text = "Deep Level"
`

func TestTscnParser(t *testing.T) {
	// Create temporary test file
	tempFile := "test_temp.tscn"
	err := os.WriteFile(tempFile, []byte(testTscnContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tempFile)

	// Test parser
	scene, err := ParseTscnFile(tempFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Basic checks
	if len(scene.AllNodes) != 5 {
		t.Errorf("Expected 5 nodes, got: %d", len(scene.AllNodes))
	}

	if scene.RootNode.OriginalName != "Root" {
		t.Errorf("Expected root node: Root, got: %s", scene.RootNode.OriginalName)
	}

	// Structure checks
	expectedStructure := map[string][]string{
		"Root":       {"Child1", "Child2"},
		"Child1":     {"GrandChild"},
		"GrandChild": {"DeepChild"},
	}

	for parentName, expectedChildren := range expectedStructure {
		var parentNode *GodotNode
		for _, node := range scene.AllNodes {
			if node.OriginalName == parentName {
				parentNode = node
				break
			}
		}

		if parentNode == nil {
			t.Errorf("Node %s not found", parentName)
			continue
		}

		if len(parentNode.Children) != len(expectedChildren) {
			t.Errorf("%s has wrong number of children (expected: %d, got: %d)",
				parentName, len(expectedChildren), len(parentNode.Children))
			continue
		}

		for i, expectedChild := range expectedChildren {
			if parentNode.Children[i].OriginalName != expectedChild {
				t.Errorf("%s child node %d is wrong (expected: %s, got: %s)",
					parentName, i, expectedChild, parentNode.Children[i].OriginalName)
			}
		}
	}

	// Property checks
	for _, node := range scene.AllNodes {
		if node.OriginalName == "GrandChild" {
			if text, exists := node.Properties["text"]; exists {
				expected := "\"Test Button\""
				if text != expected {
					t.Errorf("GrandChild text property is wrong (expected: %s, got: %s)", expected, text)
				}
			} else {
				t.Error("GrandChild text property not found")
			}
		}

		if node.OriginalName == "DeepChild" {
			if text, exists := node.Properties["text"]; exists {
				expected := "\"Deep Level\""
				if text != expected {
					t.Errorf("DeepChild text property is wrong (expected: %s, got: %s)", expected, text)
				}
			} else {
				t.Error("DeepChild text property not found")
			}
		}
	}
}


func TestMultilineTextParsing(t *testing.T) {
	multilineContent := `[gd_scene load_steps=1 format=3]

[node name="Root" type="Node2D"]

[node name="TestLabel" type="RichTextLabel" parent="."]
text = "★3
Ceylon"
`

	tempFile := "test_multiline.tscn"
	err := os.WriteFile(tempFile, []byte(multilineContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tempFile)

	scene, err := ParseTscnFile(tempFile)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	var testLabel *GodotNode
	for _, node := range scene.AllNodes {
		if node.OriginalName == "TestLabel" {
			testLabel = node
			break
		}
	}

	if testLabel == nil {
		t.Fatal("TestLabel not found")
	}

	text, exists := testLabel.Properties["text"]
	if !exists {
		t.Fatal("text property not found")
	}

	expected := "★3\nCeylon"
	if text != expected {
		t.Errorf("Multiline text not parsed correctly (expected: %q, got: %q)", expected, text)
	}
}