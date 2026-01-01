# Godot Scene Parser

Godotのtscnファイルをパースしてシーンツリーの状態を表示するGoツールです。

## 機能

- **tscnファイル解析**: Godotシーンファイルの構造を解析
- **シーンツリー表示**: ノードの階層構造を視覚的に表示
- **ノード情報表示**: 各ノードのタイプ、プロパティ、スクリプト情報
- **統計情報**: シーンの統計データ（ノード数、タイプ別集計など）
- **複数ファイル対応**: 複数のtscnファイルを一度に解析

## インストール

```bash
cd bin
go build -o godot-scene-parser main.go
```

## 使用方法

### 単一ファイル解析
```bash
./godot-scene-parser main.tscn
```

### 複数ファイル解析
```bash
./godot-scene-parser main.tscn player.tscn enemy.tscn
```

### Windows
```bash
godot-scene-parser.exe main.tscn
```

## 出力例

```
📂 Godotシーンパーサー
ファイル: main.tscn

=== シーン統計 ===
形式バージョン: 3
読み込みステップ: 5
総ノード数: 8
リソース数: 3
スクリプト付きノード: 2

ノードタイプ別:
  📁 Node: 1個
  🏃 CharacterBody2D: 1個
  🖼️ Sprite2D: 2個
  🛡️ CollisionShape2D: 1個
  📷 Camera2D: 1個
  ⬜ Control: 2個

=== シーンツリー ===
📁 Main (Node)
  🏃 Player (CharacterBody2D) [スクリプト: ExtResource("1_abc123")]
    position: Vector2(100, 200)
    🖼️ Sprite (Sprite2D)
      texture: ExtResource("2_def456")
    🛡️ CollisionShape2D (CollisionShape2D)
  📷 Camera (Camera2D)
  ⬜ UI (Control)
    ⬜ HealthBar (Control)
```

## 対応ノードタイプ

以下のノードタイプに専用アイコンを用意：

- **基本ノード**: Node(📁), Node2D(🔵), Node3D(🎯), Control(⬜)
- **物理ノード**: CharacterBody2D(🏃), RigidBody2D(⚽), Area2D(📡)
- **ビジュアルノード**: Sprite2D(🖼️), Label(📝), Button(🔘)
- **コンテナノード**: VBoxContainer(📦), HBoxContainer(📦)
- **その他**: Camera2D(📷), Timer(⏰), AudioStreamPlayer(🔊)

## 表示される情報

### ノード情報
- ノード名とタイプ
- アタッチされたスクリプト
- 重要なプロパティ（position, scale, texture など）

### 統計情報
- シーン形式バージョン
- 総ノード数
- ノードタイプ別集計
- スクリプト付きノード数
- リソース数

## 開発者向け

### 構造体
- `GodotNode`: ノード情報
- `GodotScene`: シーン全体の情報

### 主要関数
- `parseTscnFile()`: tscnファイルの解析
- `buildSceneTree()`: シーンツリーの構築
- `printSceneTree()`: ツリー表示
- `printSceneStats()`: 統計表示

### 拡張可能
- 新しいノードタイプのアイコン追加
- 追加プロパティの表示
- 出力形式の変更（JSON、XML等）