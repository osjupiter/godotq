package main

import (
	"os"
	"testing"
)

// テスト用のシンプルなtscnファイル内容
const testTscnContent = `[gd_scene load_steps=2 format=3]

[node name="Root" type="Node2D"]

[node name="Child1" type="Control" parent="."]

[node name="Child2" type="Control" parent="."]

[node name="GrandChild" type="Button" parent="Child1"]
text = "テストボタン"

[node name="DeepChild" type="Label" parent="Child1/GrandChild"]
text = "ディープレベル"
`

func TestTscnParser(t *testing.T) {
	// テスト用のtempファイルを作成
	tempFile := "test_temp.tscn"
	err := os.WriteFile(tempFile, []byte(testTscnContent), 0644)
	if err != nil {
		t.Fatalf("テストファイル作成エラー: %v", err)
	}
	defer os.Remove(tempFile)

	// パーサーをテスト
	scene, err := ParseTscnFile(tempFile)
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	// 基本チェック
	if len(scene.AllNodes) != 5 {
		t.Errorf("期待されるノード数: 5, 実際: %d", len(scene.AllNodes))
	}

	if scene.RootNode.OriginalName != "Root" {
		t.Errorf("期待されるルートノード: Root, 実際: %s", scene.RootNode.OriginalName)
	}

	// 構造チェック
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
			t.Errorf("ノード %s が見つかりません", parentName)
			continue
		}

		if len(parentNode.Children) != len(expectedChildren) {
			t.Errorf("%s の子ノード数が間違っています (期待: %d, 実際: %d)",
				parentName, len(expectedChildren), len(parentNode.Children))
			continue
		}

		for i, expectedChild := range expectedChildren {
			if parentNode.Children[i].OriginalName != expectedChild {
				t.Errorf("%s の子ノード %d が間違っています (期待: %s, 実際: %s)",
					parentName, i, expectedChild, parentNode.Children[i].OriginalName)
			}
		}
	}

	// プロパティチェック
	for _, node := range scene.AllNodes {
		if node.OriginalName == "GrandChild" {
			if text, exists := node.Properties["text"]; exists {
				expected := "\"テストボタン\""
				if text != expected {
					t.Errorf("GrandChildのtextプロパティが間違っています (期待: %s, 実際: %s)", expected, text)
				}
			} else {
				t.Error("GrandChildのtextプロパティが見つかりません")
			}
		}

		if node.OriginalName == "DeepChild" {
			if text, exists := node.Properties["text"]; exists {
				expected := "\"ディープレベル\""
				if text != expected {
					t.Errorf("DeepChildのtextプロパティが間違っています (期待: %s, 実際: %s)", expected, text)
				}
			} else {
				t.Error("DeepChildのtextプロパティが見つかりません")
			}
		}
	}
}

func TestMainTscnStructure(t *testing.T) {
	// ファイルが存在しない場合はスキップ
	if _, err := os.Stat("../main.tscn"); os.IsNotExist(err) {
		t.Skip("../main.tscn が見つかりません。テストをスキップします。")
	}

	scene, err := ParseTscnFile("../main.tscn")
	if err != nil {
		t.Fatalf("main.tscnパースエラー: %v", err)
	}

	// 基本チェック
	if len(scene.AllNodes) == 0 {
		t.Fatal("ノードが見つかりません")
	}

	if len(scene.Resources) == 0 {
		t.Error("リソースが見つかりません")
	}

	// メインのControlノードを見つける
	var mainControl *GodotNode
	for _, node := range scene.AllNodes {
		if node.OriginalName == "Control" && node.Parent == "." {
			mainControl = node
			break
		}
	}

	if mainControl == nil {
		t.Fatal("メインのControlノードが見つかりません")
	}

	// 各シーンノードがメインControlの直接の子として存在するかチェック
	expectedScenes := []string{"missionScene", "scrapScene", "partyScene", "battleScene"}

	for _, sceneName := range expectedScenes {
		found := false
		for _, child := range mainControl.Children {
			if child.OriginalName == sceneName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s がメインControlの子として見つかりません", sceneName)
		}
	}

	// battleSceneの子ノードをチェック
	var battleScene *GodotNode
	for _, child := range mainControl.Children {
		if child.OriginalName == "battleScene" {
			battleScene = child
			break
		}
	}

	if battleScene == nil {
		t.Error("battleSceneが見つかりません")
	} else {
		if len(battleScene.Children) < 2 {
			t.Errorf("battleSceneの子ノード数が不足しています (期待: 2以上, 実際: %d)", len(battleScene.Children))
		}
	}
}

func TestMultilineTextParsing(t *testing.T) {
	multilineContent := `[gd_scene load_steps=1 format=3]

[node name="Root" type="Node2D"]

[node name="TestLabel" type="RichTextLabel" parent="."]
text = "★3
セイロン"
`

	tempFile := "test_multiline.tscn"
	err := os.WriteFile(tempFile, []byte(multilineContent), 0644)
	if err != nil {
		t.Fatalf("テストファイル作成エラー: %v", err)
	}
	defer os.Remove(tempFile)

	scene, err := ParseTscnFile(tempFile)
	if err != nil {
		t.Fatalf("パースエラー: %v", err)
	}

	var testLabel *GodotNode
	for _, node := range scene.AllNodes {
		if node.OriginalName == "TestLabel" {
			testLabel = node
			break
		}
	}

	if testLabel == nil {
		t.Fatal("TestLabelが見つかりません")
	}

	text, exists := testLabel.Properties["text"]
	if !exists {
		t.Fatal("textプロパティが見つかりません")
	}

	expected := "★3\nセイロン"
	if text != expected {
		t.Errorf("マルチラインテキストが正しく解析されていません (期待: %q, 実際: %q)", expected, text)
	}
}