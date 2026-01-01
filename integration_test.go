package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// サブモジュールが初期化されているか確認
func checkSubmoduleInitialized(t *testing.T) bool {
	demoProjectsPath := "test/godot-demo-projects"
	if _, err := os.Stat(demoProjectsPath); os.IsNotExist(err) {
		t.Skip("godot-demo-projects サブモジュールが初期化されていません。'git submodule update --init' を実行してください。")
		return false
	}
	return true
}

// godot-demo-projects内の全tscnファイルを取得
func findTscnFiles(rootPath string) ([]string, error) {
	var tscnFiles []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".tscn") {
			tscnFiles = append(tscnFiles, path)
		}

		return nil
	})

	return tscnFiles, err
}

// 全デモプロジェクトのtscnファイルをパース
func TestGodotDemoProjects(t *testing.T) {
	if !checkSubmoduleInitialized(t) {
		return
	}

	demoProjectsPath := "test/godot-demo-projects"

	t.Logf("デモプロジェクトディレクトリを検索: %s", demoProjectsPath)

	tscnFiles, err := findTscnFiles(demoProjectsPath)
	if err != nil {
		t.Fatalf("tscnファイル検索エラー: %v", err)
	}

	if len(tscnFiles) == 0 {
		t.Fatal("tscnファイルが見つかりませんでした")
	}

	t.Logf("検出されたtscnファイル数: %d", len(tscnFiles))

	successCount := 0
	failCount := 0
	var failedFiles []string

	for _, file := range tscnFiles {
		t.Run(file, func(t *testing.T) {
			scene, err := ParseTscnFile(file)
			if err != nil {
				failCount++
				failedFiles = append(failedFiles, file)
				t.Errorf("パースエラー: %v", err)
				return
			}

			// 基本的な妥当性チェック
			if scene == nil {
				failCount++
				failedFiles = append(failedFiles, file)
				t.Error("シーンがnilです")
				return
			}

			// ノードが少なくとも1つあることを確認
			if len(scene.AllNodes) == 0 {
				t.Logf("警告: ノードが見つかりませんでした（空のシーンの可能性）")
			}

			successCount++
		})
	}

	// サマリー表示
	t.Logf("\n=== テスト結果サマリー ===")
	t.Logf("総ファイル数: %d", len(tscnFiles))
	t.Logf("成功: %d", successCount)
	t.Logf("失敗: %d", failCount)

	if len(failedFiles) > 0 {
		t.Logf("\n失敗したファイル:")
		for _, file := range failedFiles {
			t.Logf("  - %s", file)
		}
	}

	// 成功率をチェック
	successRate := float64(successCount) / float64(len(tscnFiles)) * 100
	t.Logf("成功率: %.2f%%", successRate)

	// 成功率が80%未満の場合は警告
	if successRate < 80.0 {
		t.Errorf("成功率が低すぎます (%.2f%% < 80%%)", successRate)
	}
}

// 特定のデモプロジェクトを詳細にテスト
func TestSpecificDemoProjects(t *testing.T) {
	if !checkSubmoduleInitialized(t) {
		return
	}

	testCases := []struct {
		name         string
		path         string
		minNodes     int
		shouldHaveRoot bool
	}{
		{
			name:         "Dodge the Creeps - Main Scene",
			path:         "test/godot-demo-projects/2d/dodge_the_creeps/main.tscn",
			minNodes:     1,
			shouldHaveRoot: true,
		},
		{
			name:         "Dodge the Creeps - Player",
			path:         "test/godot-demo-projects/2d/dodge_the_creeps/player.tscn",
			minNodes:     1,
			shouldHaveRoot: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// ファイル存在チェック
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skipf("ファイルが見つかりません: %s", tc.path)
				return
			}

			scene, err := ParseTscnFile(tc.path)
			if err != nil {
				t.Fatalf("パースエラー: %v", err)
			}

			// ノード数チェック
			if len(scene.AllNodes) < tc.minNodes {
				t.Errorf("ノード数が期待より少ない (期待: %d以上, 実際: %d)",
					tc.minNodes, len(scene.AllNodes))
			}

			// ルートノードチェック
			if tc.shouldHaveRoot && scene.RootNode == nil {
				t.Error("ルートノードが見つかりません")
			}

			if scene.RootNode != nil {
				t.Logf("ルートノード: %s (%s)", scene.RootNode.OriginalName, scene.RootNode.Type)
				t.Logf("総ノード数: %d", len(scene.AllNodes))
				t.Logf("子ノード数: %d", len(scene.RootNode.Children))
			}
		})
	}
}

// パフォーマンステスト（オプション）
func TestParsingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("パフォーマンステストをスキップ (-short フラグ)")
	}

	if !checkSubmoduleInitialized(t) {
		return
	}

	demoProjectsPath := "test/godot-demo-projects"
	tscnFiles, err := findTscnFiles(demoProjectsPath)
	if err != nil {
		t.Fatalf("tscnファイル検索エラー: %v", err)
	}

	if len(tscnFiles) == 0 {
		t.Skip("tscnファイルが見つかりませんでした")
	}

	// 最初の10ファイルでパフォーマンステスト
	testFiles := tscnFiles
	if len(testFiles) > 10 {
		testFiles = tscnFiles[:10]
	}

	for _, file := range testFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			_, err := ParseTscnFile(file)
			if err != nil {
				t.Logf("パースエラー (パフォーマンステストのため継続): %v", err)
			}
		})
	}
}
