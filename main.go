package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// デバッグモード
var debugMode = false

// ノード情報を表す構造体
type GodotNode struct {
	Name         string
	OriginalName string
	Type         string
	Parent       string
	Index        int
	Path         string
	Script       string
	Properties   map[string]string
	Children     []*GodotNode
}

// リソース情報を表す構造体
type GodotResource struct {
	ID   string
	Type string
	Path string
	UID  string
}

// シーン情報を表す構造体
type GodotScene struct {
	Version       string
	LoadSteps     int
	Format        int
	RootNode      *GodotNode
	AllNodes      []*GodotNode
	Resources     []string
	Extensions    []string
	ExtResources  map[string]*GodotResource
	SubResources  map[string]*GodotResource
}

// デバッグログ
func debugLog(msg string, args ...interface{}) {
	if debugMode {
		fmt.Printf("[DEBUG] "+msg+"\n", args...)
	}
}

// ParseTscnFile tscnファイルをパースする
func ParseTscnFile(filepath string) (*GodotScene, error) {
	debugLog("ファイルを開いています: %s", filepath)

	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("ファイルを開けませんでした: %v", err)
	}
	defer file.Close()

	scene := &GodotScene{
		AllNodes:     make([]*GodotNode, 0),
		Resources:    make([]string, 0),
		Extensions:   make([]string, 0),
		ExtResources: make(map[string]*GodotResource),
		SubResources: make(map[string]*GodotResource),
	}

	scanner := bufio.NewScanner(file)
	// バッファサイズを増やして大きなファイルに対応（最大10MB）
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var currentNode *GodotNode
	var inNode bool
	var multilineProperty string
	var multilineValue strings.Builder
	var inMultiline bool
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		originalLine := scanner.Text()

		debugLog("行 %d: %s", lineNum, originalLine)

		// マルチライン処理
		if inMultiline {
			if strings.HasSuffix(line, "\"") {
				// マルチラインの終了
				multilineValue.WriteString(strings.TrimSuffix(line, "\""))
				if currentNode != nil {
					currentNode.Properties[multilineProperty] = multilineValue.String()
					if multilineProperty == "script" {
						currentNode.Script = multilineValue.String()
					}
				}
				inMultiline = false
				multilineProperty = ""
				multilineValue.Reset()
				continue
			} else {
				// マルチラインの継続
				multilineValue.WriteString(line + "\n")
				continue
			}
		}

		// 空行やコメントをスキップ
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// ヘッダー情報をパース
		if strings.HasPrefix(line, "[gd_scene") {
			debugLog("ヘッダーをパース: %s", line)
			parseHeader(line, scene)
			inNode = false
			continue
		}

		// リソース情報をパース
		if strings.HasPrefix(line, "[ext_resource") || strings.HasPrefix(line, "[sub_resource") {
			debugLog("リソースをパース: %s", line)
			parseResource(line, scene)
			inNode = false
			continue
		}

		// ノード開始
		if strings.HasPrefix(line, "[node") {
			debugLog("ノード開始: %s", line)
			if currentNode != nil {
				debugLog("前のノードを追加: %s (%s)", currentNode.Name, currentNode.Type)
				scene.AllNodes = append(scene.AllNodes, currentNode)
			}
			currentNode = parseNodeHeader(line)
			if currentNode != nil {
				debugLog("新しいノード作成: %s (%s) parent=%s", currentNode.Name, currentNode.Type, currentNode.Parent)
			}
			inNode = true
			continue
		}

		// その他のセクション開始（コネクションなど）
		if strings.HasPrefix(line, "[") {
			debugLog("その他のセクション: %s", line)
			inNode = false
			continue
		}

		// ノード内のプロパティ
		if inNode && currentNode != nil {
			debugLog("プロパティをパース: %s", line)
			// マルチラインの開始チェック
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					if strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
						// マルチラインの開始
						inMultiline = true
						multilineProperty = key
						multilineValue.WriteString(strings.TrimPrefix(value, "\"") + "\n")
						continue
					}
				}
			}
			parseNodeProperty(line, currentNode)
		}
	}

	// 最後のノードを追加
	if currentNode != nil {
		debugLog("最後のノードを追加: %s (%s)", currentNode.Name, currentNode.Type)
		scene.AllNodes = append(scene.AllNodes, currentNode)
	}

	debugLog("パース完了。ノード総数: %d", len(scene.AllNodes))

	// シーンツリーを構築
	buildSceneTree(scene)

	return scene, scanner.Err()
}

// ヘッダー情報をパース
func parseHeader(line string, scene *GodotScene) {
	// [gd_scene load_steps=3 format=3]
	re := regexp.MustCompile(`load_steps=(\d+)`)
	if matches := re.FindStringSubmatch(line); len(matches) > 1 {
		scene.LoadSteps, _ = strconv.Atoi(matches[1])
	}

	re = regexp.MustCompile(`format=(\d+)`)
	if matches := re.FindStringSubmatch(line); len(matches) > 1 {
		scene.Format, _ = strconv.Atoi(matches[1])
	}
}

// リソース情報をパース
func parseResource(line string, scene *GodotScene) {
	scene.Resources = append(scene.Resources, line)

	if strings.HasPrefix(line, "[ext_resource") {
		parseExtResource(line, scene)
	} else if strings.HasPrefix(line, "[sub_resource") {
		parseSubResource(line, scene)
	}
}

// ExtResourceをパース
func parseExtResource(line string, scene *GodotScene) {
	resource := &GodotResource{}

	// type="Script" を抽出
	typeRe := regexp.MustCompile(`type="([^"]*)"`)
	if matches := typeRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.Type = matches[1]
	}

	// path="res://..." を抽出
	pathRe := regexp.MustCompile(`path="([^"]*)"`)
	if matches := pathRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.Path = matches[1]
	}

	// id="1_abc123" を抽出（これが実際の参照で使われるID）
	idRe := regexp.MustCompile(`\bid="([^"]*)"`)
	if matches := idRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.ID = matches[1]
	}

	// uid="uid://..." を抽出
	uidRe := regexp.MustCompile(`uid="([^"]*)"`)
	if matches := uidRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.UID = matches[1]
	}

	// IDがある場合に保存（IDが実際の参照キー）
	if resource.ID != "" {
		scene.ExtResources[resource.ID] = resource
		debugLog("ExtResource追加: %s (%s) -> %s", resource.ID, resource.Type, resource.Path)
	} else if resource.UID != "" {
		// IDがない場合はUIDを使用
		scene.ExtResources[resource.UID] = resource
		debugLog("ExtResource追加: %s (%s) -> %s", resource.UID, resource.Type, resource.Path)
	}
}

// SubResourceをパース
func parseSubResource(line string, scene *GodotScene) {
	resource := &GodotResource{}

	// type="CanvasTexture" を抽出
	typeRe := regexp.MustCompile(`type="([^"]*)"`)
	if matches := typeRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.Type = matches[1]
	}

	// id="CanvasTexture_38dae" を抽出
	idRe := regexp.MustCompile(`id="([^"]*)"`)
	if matches := idRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.ID = matches[1]
	}

	if resource.ID != "" {
		scene.SubResources[resource.ID] = resource
		debugLog("SubResource追加: %s (%s)", resource.ID, resource.Type)
	}
}

// ノードヘッダーをパース
func parseNodeHeader(line string) *GodotNode {
	node := &GodotNode{
		Properties: make(map[string]string),
		Children:   make([]*GodotNode, 0),
	}

	// [node name="Player" type="CharacterBody2D" parent="."]
	re := regexp.MustCompile(`name="([^"]*)"`)
	if matches := re.FindStringSubmatch(line); len(matches) > 1 {
		node.Name = matches[1]
	}

	re = regexp.MustCompile(`type="([^"]*)"`)
	if matches := re.FindStringSubmatch(line); len(matches) > 1 {
		node.Type = matches[1]
	}

	re = regexp.MustCompile(`parent="([^"]*)"`)
	if matches := re.FindStringSubmatch(line); len(matches) > 1 {
		node.Parent = matches[1]
	}

	re = regexp.MustCompile(`index="(\d+)"`)
	if matches := re.FindStringSubmatch(line); len(matches) > 1 {
		node.Index, _ = strconv.Atoi(matches[1])
	}

	return node
}

// ノードプロパティをパース
func parseNodeProperty(line string, node *GodotNode) {
	// script = ExtResource("1_abc123")
	if strings.Contains(line, "=") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// 複数行テキストの処理
			if strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
				// 複数行の開始
				value = strings.TrimPrefix(value, "\"")
			} else if strings.HasSuffix(value, "\"") && !strings.HasPrefix(value, "\"") {
				// 複数行の終了
				value = strings.TrimSuffix(value, "\"")
			}

			// 改行文字を保持
			value = strings.ReplaceAll(value, "\\n", "\n")

			node.Properties[key] = value

			// 特別なプロパティを処理
			if key == "script" {
				node.Script = value
			}
		}
	}
}

// シーンツリーを構築
func buildSceneTree(scene *GodotScene) {
	debugLog("シーンツリー構築開始")

	pathMap := make(map[string]*GodotNode)

	// 順次処理で親子関係を構築（コンテキストを維持）
	for i, node := range scene.AllNodes {
		// オリジナル名を保存
		node.OriginalName = node.Name

		debugLog("ノード処理: %s (parent: %s)", node.Name, node.Parent)

		// 親ノードを決定
		var parentNode *GodotNode
		if node.Parent == "" || node.Parent == "." {
			// ルートノードまたはルートの直接の子
			if scene.RootNode == nil && node.Parent == "" {
				// 最初のノードをルートとする
				scene.RootNode = node
				node.Path = node.Name
				pathMap[node.Path] = node
				debugLog("ルートノード設定: %s", node.Name)
				continue
			} else if node.Parent == "." && scene.RootNode != nil {
				// ルートの直接の子
				parentNode = scene.RootNode
			}
		} else {
			// 親ノードを検索（既に処理されたノードの中から）
			parentNode = findParentInProcessedNodes(node.Parent, pathMap, scene.AllNodes[:i])
		}

		// 親ノードが見つかった場合
		if parentNode != nil {
			debugLog("親ノード見つかりました: %s -> %s", node.Name, parentNode.OriginalName)
			parentNode.Children = append(parentNode.Children, node)
			node.Path = parentNode.Path + "/" + node.Name
		} else {
			// 親ノードが見つからない場合、ルートの子として扱う
			debugLog("親ノードが見つからないため、ルートの子として処理: %s", node.Name)
			if scene.RootNode != nil {
				scene.RootNode.Children = append(scene.RootNode.Children, node)
				node.Path = scene.RootNode.Path + "/" + node.Name
			} else {
				// ルートノードが未設定の場合、このノードをルートとする
				scene.RootNode = node
				node.Path = node.Name
			}
		}

		pathMap[node.Path] = node
		debugLog("パス設定: %s -> %s", node.Name, node.Path)
	}


	debugLog("シーンツリー構築完了")
}

// 処理済みノードの中から親ノードを検索
func findParentInProcessedNodes(parentPath string, pathMap map[string]*GodotNode, processedNodes []*GodotNode) *GodotNode {
	debugLog("処理済みノードから親検索: %s", parentPath)

	// 完全なパスで検索
	if parentNode, exists := pathMap[parentPath]; exists {
		debugLog("完全パスマッチ: %s", parentPath)
		return parentNode
	}

	// 単純な名前で検索（処理済みノードの中から最初に見つかったもの）
	// 処理順序に従って最初に見つかったものを優先
	for _, node := range processedNodes {
		if node.OriginalName == parentPath {
			debugLog("名前マッチ（順次）: %s -> %s", parentPath, node.Path)
			return node
		}
	}

	// 複雑なパスの場合
	if strings.Contains(parentPath, "/") {
		parts := strings.Split(parentPath, "/")
		parentName := parts[len(parts)-1]

		// 処理順序に従って最初に見つかったものを優先
		for _, node := range processedNodes {
			if node.OriginalName == parentName {
				debugLog("名前マッチ: %s -> %s", parentName, node.Path)
				return node
			}
		}
	}

	// パスの末尾を基準に検索（最後の手段）
	for path, node := range pathMap {
		if strings.HasSuffix(path, "/"+parentPath) {
			debugLog("末尾マッチ: %s -> %s", parentPath, path)
			return node
		}
	}

	return nil
}

// 親ノードを検索するヘルパー関数
func findParentNode(parentPath string, pathMap, nodeMap map[string]*GodotNode, currentNodePath string) *GodotNode {
	debugLog("親ノード検索: %s (current: %s)", parentPath, currentNodePath)

	// 完全なパスで検索（最優先）
	if parentNode, exists := pathMap[parentPath]; exists {
		debugLog("完全パスマッチ: %s", parentPath)
		return parentNode
	}

	// 複雑なパスの場合
	if strings.Contains(parentPath, "/") {
		// より具体的なパスマッチングを行う
		for path, node := range pathMap {
			if strings.HasSuffix(path, parentPath) {
				debugLog("部分パスマッチ: %s -> %s", parentPath, path)
				return node
			}
		}

		// 段階的にパスをマッチング
		parts := strings.Split(parentPath, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			testPath := strings.Join(parts[i:], "/")
			if parentNode, exists := pathMap[testPath]; exists {
				debugLog("段階的パスマッチ: %s -> %s", parentPath, testPath)
				return parentNode
			}
		}

		// 最後の要素だけで検索（最後の手段）
		parentName := parts[len(parts)-1]
		debugLog("複雑なパスを単純化: %s -> %s", parentPath, parentName)

		// 名前でマッチしたノードが複数ある場合、階層的に最も近いものを選ぶ
		var bestMatch *GodotNode
		for path, node := range pathMap {
			if strings.HasSuffix(path, "/"+parentName) || node.OriginalName == parentName {
				if bestMatch == nil {
					bestMatch = node
				} else {
					// より短いパス（より上位の階層）を優先
					if len(node.Path) < len(bestMatch.Path) {
						bestMatch = node
					}
				}
			}
		}

		if bestMatch != nil {
			debugLog("最適マッチ選択: %s -> %s", parentName, bestMatch.Path)
			return bestMatch
		}
	}

	// 単純な名前で検索
	if parentNode, exists := nodeMap[parentPath]; exists {
		debugLog("名前マッチ: %s", parentPath)
		return parentNode
	}

	return nil
}

// パスでノードを検索
func findNodeByPath(nodes []*GodotNode, path string) *GodotNode {
	for _, node := range nodes {
		if node.Path == path || node.Name == path {
			return node
		}
	}
	return nil
}

// シーンツリーを表示
func printSceneTree(node *GodotNode, indent int, scene *GodotScene) {
	if node == nil {
		return
	}

	indentStr := strings.Repeat("  ", indent)
	icon := getNodeIcon(node.Type)

	fmt.Printf("%s%s %s (%s)", indentStr, icon, node.OriginalName, node.Type)

	if node.Script != "" {
		scriptPath := resolveResourcePath(node.Script, scene)
		if scriptPath != "" {
			fmt.Printf(" [スクリプト: %s]", scriptPath)
		} else {
			fmt.Printf(" [スクリプト: %s]", node.Script)
		}
	}

	fmt.Println()

	// 重要なプロパティを表示
	if len(node.Properties) > 0 {
		showImportantProperties(node, indent+1, scene)
	}

	// 子ノードを再帰的に表示
	for _, child := range node.Children {
		printSceneTree(child, indent+1, scene)
	}
}

// ノードタイプに応じたアイコンを返す
func getNodeIcon(nodeType string) string {
	icons := map[string]string{
		"Node":              "📁",
		"Node2D":            "🔵",
		"Node3D":            "🎯",
		"Control":           "⬜",
		"CanvasLayer":       "🖼️",
		"CharacterBody2D":   "🏃",
		"RigidBody2D":       "⚽",
		"Area2D":            "📡",
		"StaticBody2D":      "🧱",
		"Sprite2D":          "🖼️",
		"AnimatedSprite2D":  "🎬",
		"Label":             "📝",
		"Button":            "🔘",
		"TextEdit":          "📄",
		"Panel":             "📋",
		"VBoxContainer":     "📦",
		"HBoxContainer":     "📦",
		"GridContainer":     "🔲",
		"ScrollContainer":   "📜",
		"Camera2D":          "📷",
		"AudioStreamPlayer": "🔊",
		"Timer":             "⏰",
		"AnimationPlayer":   "▶️",
		"CollisionShape2D":  "🛡️",
	}

	if icon, exists := icons[nodeType]; exists {
		return icon
	}
	return "❓"
}

// 重要なプロパティを表示
func showImportantProperties(node *GodotNode, indent int, scene *GodotScene) {
	indentStr := strings.Repeat("  ", indent)
	importantProps := []string{"position", "scale", "rotation", "size", "text", "texture", "visible"}

	for _, prop := range importantProps {
		if value, exists := node.Properties[prop]; exists {
			if prop == "texture" {
				// テクスチャリソースを解決
				texturePath := resolveResourcePath(value, scene)
				if texturePath != "" {
					fmt.Printf("%s  %s: %s\n", indentStr, prop, texturePath)
				} else {
					fmt.Printf("%s  %s: %s\n", indentStr, prop, value)
				}
			} else {
				fmt.Printf("%s  %s: %s\n", indentStr, prop, value)
			}
		}
	}
}

// リソース参照を実際のパスに解決
func resolveResourcePath(resourceRef string, scene *GodotScene) string {
	// ExtResource("1_abc123") の形式を解析
	extResourceRe := regexp.MustCompile(`ExtResource\("([^"]*)"\)`)
	if matches := extResourceRe.FindStringSubmatch(resourceRef); len(matches) > 1 {
		resourceID := matches[1]
		if resource, exists := scene.ExtResources[resourceID]; exists {
			return resource.Path
		}
	}

	// SubResource("SubResource_123") の形式を解析
	subResourceRe := regexp.MustCompile(`SubResource\("([^"]*)"\)`)
	if matches := subResourceRe.FindStringSubmatch(resourceRef); len(matches) > 1 {
		resourceID := matches[1]
		if resource, exists := scene.SubResources[resourceID]; exists {
			return fmt.Sprintf("SubResource(%s)", resource.Type)
		}
	}

	return ""
}

// シーン統計を表示
func printSceneStats(scene *GodotScene) {
	fmt.Println("=== シーン統計 ===")
	fmt.Printf("形式バージョン: %d\n", scene.Format)
	fmt.Printf("読み込みステップ: %d\n", scene.LoadSteps)
	fmt.Printf("総ノード数: %d\n", len(scene.AllNodes))
	fmt.Printf("リソース数: %d\n", len(scene.Resources))

	// ノードタイプ別集計
	typeCount := make(map[string]int)
	scriptCount := 0

	for _, node := range scene.AllNodes {
		typeCount[node.Type]++
		if node.Script != "" {
			scriptCount++
		}
	}

	fmt.Printf("スクリプト付きノード: %d\n", scriptCount)

	// リソース統計
	fmt.Printf("ExtResources: %d\n", len(scene.ExtResources))
	fmt.Printf("SubResources: %d\n", len(scene.SubResources))

	fmt.Println("\nノードタイプ別:")
	for nodeType, count := range typeCount {
		icon := getNodeIcon(nodeType)
		fmt.Printf("  %s %s: %d個\n", icon, nodeType, count)
	}

	// ExtResourceタイプ別集計
	if len(scene.ExtResources) > 0 {
		fmt.Println("\nExtResourceタイプ別:")
		extTypeCount := make(map[string]int)
		for _, resource := range scene.ExtResources {
			extTypeCount[resource.Type]++
		}
		for extType, count := range extTypeCount {
			fmt.Printf("  📁 %s: %d個\n", extType, count)
		}
	}

	fmt.Println()
}

var rootCmd = &cobra.Command{
	Use:   "gdq [flags] <tscnファイル> [tscnファイル...]",
	Short: "Godotシーンファイルパーサー",
	Long:  `Godotのtscnファイルをパースしてシーンツリーの状態を表示するツールです。`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 最初のファイルを処理
		tscnFile := args[0]

		// ファイル存在チェック
		if _, err := os.Stat(tscnFile); os.IsNotExist(err) {
			return fmt.Errorf("ファイルが見つかりません: %s", tscnFile)
		}

		fmt.Printf("📂 Godotシーンパーサー\n")
		fmt.Printf("ファイル: %s\n\n", tscnFile)

		// tscnファイルをパース
		scene, err := ParseTscnFile(tscnFile)
		if err != nil {
			return fmt.Errorf("パースエラー: %v", err)
		}

		// 統計情報を表示
		printSceneStats(scene)

		// シーンツリーを表示
		fmt.Println("=== シーンツリー ===")
		if scene.RootNode != nil {
			printSceneTree(scene.RootNode, 0, scene)
		} else {
			fmt.Println("ルートノードが見つかりませんでした")
		}

		// 複数ファイル対応
		if len(args) > 1 {
			for _, file := range args[1:] {
				// ファイル存在チェック
				if _, err := os.Stat(file); os.IsNotExist(err) {
					fmt.Printf("\nエラー: ファイルが見つかりません: %s\n", file)
					continue
				}

				fmt.Printf("\n" + strings.Repeat("=", 50) + "\n")
				fmt.Printf("ファイル: %s\n\n", file)

				scene, err := ParseTscnFile(file)
				if err != nil {
					fmt.Printf("エラー: %v\n", err)
					continue
				}

				printSceneStats(scene)
				fmt.Println("=== シーンツリー ===")
				if scene.RootNode != nil {
					printSceneTree(scene.RootNode, 0, scene)
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&debugMode, "debug", "d", false, "デバッグモードを有効化")
}

// メイン関数
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
