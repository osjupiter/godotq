# gdq - Godot Scene Query Tool

A Go-based CLI tool to parse Godot .tscn files and display scene tree structures with query capabilities.

## Features

- **tscn File Parsing**: Parse Godot scene file structures
- **Scene Tree Display**: Visualize node hierarchy
- **Node Information**: Display node types, properties, and script information
- **Query Support**: Filter and search for specific nodes by path
- **Verbose Mode**: Display all node properties in detail
- **Statistics Summary**: Scene statistics (node count, type breakdown, etc.)
- **Multiple File Support**: Parse multiple tscn files at once

## Installation

```bash
go build -o gdq
```

Or on Windows:
```bash
go build -o gdq.exe
```

## Usage

### Basic Usage

Display scene tree:
```bash
./gdq main.tscn
```

### Query Specific Nodes

Search for a specific node and display its subtree:
```bash
./gdq -q Player main.tscn
./gdq -q "Player/Sprite" main.tscn
```

### Verbose Mode

Display all node properties:
```bash
./gdq -q Player -v main.tscn
```

### Statistics Summary

Display scene statistics:
```bash
./gdq -s main.tscn
```

### Multiple Files

Parse multiple files:
```bash
./gdq main.tscn player.tscn enemy.tscn
```

### Debug Mode

Enable debug logging:
```bash
./gdq -d main.tscn
```

## Command Line Flags

- `-q, --query <path>`: Search for a specific node path (e.g., "Player/Sprite")
- `-v, --verbose`: Display all properties in detail
- `-s, --summary`: Display statistics summary
- `-d, --debug`: Enable debug mode

## Output Example

### Default Output (Scene Tree)

```
Control (Control)
  missionScene (Control)
    missionSceneUI (Control)
      missionDetailRect (Panel)
  scrapScene (Control)
  partyScene (Control)
  battleScene (Control)
    battleField (Control)
    battleUI (Control)
```

### With Query (-q flag)

```bash
./gdq -q battleScene main.tscn
```

Output:
```
battleScene (Control)
  battleField (Control)
  battleUI (Control)
```

### With Verbose Mode (-q -v flags)

```bash
./gdq -q Player -v main.tscn
```

Output:
```
Player (CharacterBody2D)
  script: ExtResource("1_abc123")
  position: Vector2(100, 200)
  scale: Vector2(1, 1)
  rotation: 0.0
  Sprite (Sprite2D)
    texture: res://player.png
    scale: Vector2(0.5, 0.5)
```

### With Summary (-s flag)

```
=== Scene Statistics ===
Format Version: 3
Load Steps: 5
Total Nodes: 8
Resources: 3
Nodes with Scripts: 2
ExtResources: 3
SubResources: 2

By Node Type:
  Control: 5
  Panel: 1
  CharacterBody2D: 1
  Sprite2D: 1

By ExtResource Type:
  Script: 2
  Texture2D: 1
```

## Supported Node Types

The parser supports all Godot node types including:

- **Basic Nodes**: Node, Node2D, Node3D, Control
- **Physics Nodes**: CharacterBody2D, RigidBody2D, RigidBody3D, Area2D, Area3D
- **Visual Nodes**: Sprite2D, Sprite3D, Label, RichTextLabel, Button
- **Container Nodes**: VBoxContainer, HBoxContainer, GridContainer, ScrollContainer
- **And many more**: Camera2D, Camera3D, Timer, AudioStreamPlayer, AnimationPlayer, etc.

## Displayed Information

### Node Information
- Node name and type
- Attached scripts (with resource resolution)
- Important properties (position, scale, texture, text, etc.)
- All properties in verbose mode

### Statistics (with -s flag)
- Scene format version
- Total node count
- Node count by type
- Nodes with scripts count
- Resource count (ExtResources and SubResources)
- Resource breakdown by type

## For Developers

### Main Structures

- `GodotNode`: Represents a node in the scene
  - Properties: Name, Type, Parent, Path, Properties, Children, etc.
- `GodotResource`: Represents an external or sub-resource
  - Properties: ID, Type, Path, UID
- `GodotScene`: Represents the entire scene
  - Contains all nodes, resources, and scene metadata

### Main Functions

- `ParseTscnFile()`: Parse tscn file and build scene structure
- `buildSceneTree()`: Build parent-child relationships
- `printSceneTree()`: Display tree structure
- `printSceneStats()`: Display statistics
- `findNodeByPath()`: Search for nodes by path
- `resolveResourcePath()`: Resolve resource references to actual paths

### Key Features

- **Flexible Path Matching**: Supports exact match, suffix match, and contains match
- **Resource Resolution**: Automatically resolves ExtResource and SubResource references
- **Large File Support**: Can handle files with lines up to 10MB (for embedded particle data)
- **Multiline Property Support**: Correctly parses multiline text properties

## Testing

Run unit tests:
```bash
go test
```

Run integration tests with Godot demo projects:
```bash
git submodule update --init
go test -v
```

Run performance tests:
```bash
go test -v -run TestParsingPerformance
```

## Requirements

- Go 1.16 or later
- github.com/spf13/cobra (automatically installed via go.mod)

## License

MIT License
