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

// Debug mode flag
var debugMode = false

// Display options
var showSummary = false
var nodePath = ""
var verbose = false

// GodotNode represents a node in the Godot scene
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

// GodotResource represents a resource in the Godot scene
type GodotResource struct {
	ID   string
	Type string
	Path string
	UID  string
}

// GodotScene represents the entire Godot scene
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

// debugLog prints debug messages when debug mode is enabled
func debugLog(msg string, args ...interface{}) {
	if debugMode {
		fmt.Printf("[DEBUG] "+msg+"\n", args...)
	}
}

// ParseTscnFile parses a Godot .tscn file
func ParseTscnFile(filepath string) (*GodotScene, error) {
	debugLog("Opening file: %s", filepath)

	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
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
	// Increase buffer size to handle large files (up to 10MB)
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

		debugLog("Line %d: %s", lineNum, originalLine)

		// Handle multiline properties
		if inMultiline {
			if strings.HasSuffix(line, "\"") {
				// End of multiline
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
				// Continue multiline
				multilineValue.WriteString(line + "\n")
				continue
			}
		}

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// Parse header information
		if strings.HasPrefix(line, "[gd_scene") {
			debugLog("Parsing header: %s", line)
			parseHeader(line, scene)
			inNode = false
			continue
		}

		// Parse resource information
		if strings.HasPrefix(line, "[ext_resource") || strings.HasPrefix(line, "[sub_resource") {
			debugLog("Parsing resource: %s", line)
			parseResource(line, scene)
			inNode = false
			continue
		}

		// Node start
		if strings.HasPrefix(line, "[node") {
			debugLog("Node start: %s", line)
			if currentNode != nil {
				debugLog("Adding previous node: %s (%s)", currentNode.Name, currentNode.Type)
				scene.AllNodes = append(scene.AllNodes, currentNode)
			}
			currentNode = parseNodeHeader(line)
			if currentNode != nil {
				debugLog("Created new node: %s (%s) parent=%s", currentNode.Name, currentNode.Type, currentNode.Parent)
			}
			inNode = true
			continue
		}

		// Other sections (connections, etc.)
		if strings.HasPrefix(line, "[") {
			debugLog("Other section: %s", line)
			inNode = false
			continue
		}

		// Properties within a node
		if inNode && currentNode != nil {
			debugLog("Parsing property: %s", line)
			// Check for multiline start
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					if strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
						// Multiline start
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

	// Add the last node
	if currentNode != nil {
		debugLog("Adding last node: %s (%s)", currentNode.Name, currentNode.Type)
		scene.AllNodes = append(scene.AllNodes, currentNode)
	}

	debugLog("Parsing complete. Total nodes: %d", len(scene.AllNodes))

	// Build scene tree
	buildSceneTree(scene)

	return scene, scanner.Err()
}

// parseHeader parses the scene header
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

// parseResource parses resource information
func parseResource(line string, scene *GodotScene) {
	scene.Resources = append(scene.Resources, line)

	if strings.HasPrefix(line, "[ext_resource") {
		parseExtResource(line, scene)
	} else if strings.HasPrefix(line, "[sub_resource") {
		parseSubResource(line, scene)
	}
}

// parseExtResource parses external resources
func parseExtResource(line string, scene *GodotScene) {
	resource := &GodotResource{}

	// Extract type="Script"
	typeRe := regexp.MustCompile(`type="([^"]*)"`)
	if matches := typeRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.Type = matches[1]
	}

	// Extract path="res://..."
	pathRe := regexp.MustCompile(`path="([^"]*)"`)
	if matches := pathRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.Path = matches[1]
	}

	// Extract id="1_abc123" (this is the actual ID used in references)
	idRe := regexp.MustCompile(`\bid="([^"]*)"`)
	if matches := idRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.ID = matches[1]
	}

	// Extract uid="uid://..."
	uidRe := regexp.MustCompile(`uid="([^"]*)"`)
	if matches := uidRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.UID = matches[1]
	}

	// Save if ID exists (ID is the actual reference key)
	if resource.ID != "" {
		scene.ExtResources[resource.ID] = resource
		debugLog("Added ExtResource: %s (%s) -> %s", resource.ID, resource.Type, resource.Path)
	} else if resource.UID != "" {
		// Use UID if no ID
		scene.ExtResources[resource.UID] = resource
		debugLog("Added ExtResource: %s (%s) -> %s", resource.UID, resource.Type, resource.Path)
	}
}

// parseSubResource parses sub-resources
func parseSubResource(line string, scene *GodotScene) {
	resource := &GodotResource{}

	// Extract type="CanvasTexture"
	typeRe := regexp.MustCompile(`type="([^"]*)"`)
	if matches := typeRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.Type = matches[1]
	}

	// Extract id="CanvasTexture_38dae"
	idRe := regexp.MustCompile(`id="([^"]*)"`)
	if matches := idRe.FindStringSubmatch(line); len(matches) > 1 {
		resource.ID = matches[1]
	}

	if resource.ID != "" {
		scene.SubResources[resource.ID] = resource
		debugLog("Added SubResource: %s (%s)", resource.ID, resource.Type)
	}
}

// parseNodeHeader parses a node header line
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

// parseNodeProperty parses a node property line
func parseNodeProperty(line string, node *GodotNode) {
	// script = ExtResource("1_abc123")
	if strings.Contains(line, "=") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Handle multiline text
			if strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
				// Start of multiline
				value = strings.TrimPrefix(value, "\"")
			} else if strings.HasSuffix(value, "\"") && !strings.HasPrefix(value, "\"") {
				// End of multiline
				value = strings.TrimSuffix(value, "\"")
			}

			// Preserve newline characters
			value = strings.ReplaceAll(value, "\\n", "\n")

			node.Properties[key] = value

			// Handle special properties
			if key == "script" {
				node.Script = value
			}
		}
	}
}

// buildSceneTree builds the scene tree structure
func buildSceneTree(scene *GodotScene) {
	debugLog("Building scene tree")

	pathMap := make(map[string]*GodotNode)

	// Build parent-child relationships sequentially (maintaining context)
	for i, node := range scene.AllNodes {
		// Save original name
		node.OriginalName = node.Name

		debugLog("Processing node: %s (parent: %s)", node.Name, node.Parent)

		// Determine parent node
		var parentNode *GodotNode
		if node.Parent == "" || node.Parent == "." {
			// Root node or direct child of root
			if scene.RootNode == nil && node.Parent == "" {
				// Set first node as root
				scene.RootNode = node
				node.Path = node.Name
				pathMap[node.Path] = node
				debugLog("Root node set: %s", node.Name)
				continue
			} else if node.Parent == "." && scene.RootNode != nil {
				// Direct child of root
				parentNode = scene.RootNode
			}
		} else {
			// Search for parent node (among already processed nodes)
			parentNode = findParentInProcessedNodes(node.Parent, pathMap, scene.AllNodes[:i])
		}

		// If parent node found
		if parentNode != nil {
			debugLog("Parent node found: %s -> %s", node.Name, parentNode.OriginalName)
			parentNode.Children = append(parentNode.Children, node)
			node.Path = parentNode.Path + "/" + node.Name
		} else {
			// If parent not found, treat as child of root
			debugLog("Parent not found, treating as child of root: %s", node.Name)
			if scene.RootNode != nil {
				scene.RootNode.Children = append(scene.RootNode.Children, node)
				node.Path = scene.RootNode.Path + "/" + node.Name
			} else {
				// If root node not set, set this node as root
				scene.RootNode = node
				node.Path = node.Name
			}
		}

		pathMap[node.Path] = node
		debugLog("Path set: %s -> %s", node.Name, node.Path)
	}


	debugLog("Scene tree construction complete")
}

// findParentInProcessedNodes searches for parent node among processed nodes
func findParentInProcessedNodes(parentPath string, pathMap map[string]*GodotNode, processedNodes []*GodotNode) *GodotNode {
	debugLog("Searching for parent in processed nodes: %s", parentPath)

	// Search by complete path
	if parentNode, exists := pathMap[parentPath]; exists {
		debugLog("Complete path match: %s", parentPath)
		return parentNode
	}

	// Search by simple name (first found in processed nodes)
	// Prioritize first found according to processing order
	for _, node := range processedNodes {
		if node.OriginalName == parentPath {
			debugLog("Name match (sequential): %s -> %s", parentPath, node.Path)
			return node
		}
	}

	// For complex paths
	if strings.Contains(parentPath, "/") {
		parts := strings.Split(parentPath, "/")
		parentName := parts[len(parts)-1]

		// Prioritize first found according to processing order
		for _, node := range processedNodes {
			if node.OriginalName == parentName {
				debugLog("Name match: %s -> %s", parentName, node.Path)
				return node
			}
		}
	}

	// Search based on path suffix (last resort)
	for path, node := range pathMap {
		if strings.HasSuffix(path, "/"+parentPath) {
			debugLog("Suffix match: %s -> %s", parentPath, path)
			return node
		}
	}

	return nil
}

// findParentNode is a helper function to search for parent node
func findParentNode(parentPath string, pathMap, nodeMap map[string]*GodotNode, currentNodePath string) *GodotNode {
	debugLog("Searching for parent node: %s (current: %s)", parentPath, currentNodePath)

	// Search by complete path (highest priority)
	if parentNode, exists := pathMap[parentPath]; exists {
		debugLog("Complete path match: %s", parentPath)
		return parentNode
	}

	// For complex paths
	if strings.Contains(parentPath, "/") {
		// Perform more specific path matching
		for path, node := range pathMap {
			if strings.HasSuffix(path, parentPath) {
				debugLog("Partial path match: %s -> %s", parentPath, path)
				return node
			}
		}

		// Match paths stepwise
		parts := strings.Split(parentPath, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			testPath := strings.Join(parts[i:], "/")
			if parentNode, exists := pathMap[testPath]; exists {
				debugLog("Stepwise path match: %s -> %s", parentPath, testPath)
				return parentNode
			}
		}

		// Search by last element only (last resort)
		parentName := parts[len(parts)-1]
		debugLog("Simplify complex path: %s -> %s", parentPath, parentName)

		// If multiple nodes match by name, choose the hierarchically closest one
		var bestMatch *GodotNode
		for path, node := range pathMap {
			if strings.HasSuffix(path, "/"+parentName) || node.OriginalName == parentName {
				if bestMatch == nil {
					bestMatch = node
				} else {
					// Prioritize shorter path (higher hierarchy)
					if len(node.Path) < len(bestMatch.Path) {
						bestMatch = node
					}
				}
			}
		}

		if bestMatch != nil {
			debugLog("Optimal match selected: %s -> %s", parentName, bestMatch.Path)
			return bestMatch
		}
	}

	// Search by simple name
	if parentNode, exists := nodeMap[parentPath]; exists {
		debugLog("Name match: %s", parentPath)
		return parentNode
	}

	return nil
}

// findNodeByPath searches for node by path (from entire scene)
func findNodeByPath(scene *GodotScene, path string) *GodotNode {
	for _, node := range scene.AllNodes {
		// Exact match
		if node.Path == path || node.OriginalName == path {
			return node
		}
	}

	// Partial match (suffix)
	for _, node := range scene.AllNodes {
		if strings.HasSuffix(node.Path, "/"+path) {
			return node
		}
	}

	// Contained in path (more flexible search)
	for _, node := range scene.AllNodes {
		if strings.Contains(node.Path, path) {
			return node
		}
	}

	return nil
}

// getPathToNode gets the path from root to node
func getPathToNode(scene *GodotScene, targetNode *GodotNode) []*GodotNode {
	var path []*GodotNode

	// Traverse from root
	var findPath func(node *GodotNode, target *GodotNode) bool
	findPath = func(node *GodotNode, target *GodotNode) bool {
		path = append(path, node)

		if node == target {
			return true
		}

		for _, child := range node.Children {
			if findPath(child, target) {
				return true
			}
		}

		// Not found, removing
		path = path[:len(path)-1]
		return false
	}

	if scene.RootNode != nil {
		findPath(scene.RootNode, targetNode)
	}

	return path
}

// printNodeWithPath displays path and subtree of specified node
func printNodeWithPath(scene *GodotScene, targetNode *GodotNode) {


	// Display subtree under target node
	printSceneTree(targetNode, 0, scene)
}

// printSceneTree displays the scene tree
func printSceneTree(node *GodotNode, indent int, scene *GodotScene) {
	if node == nil {
		return
	}

	indentStr := strings.Repeat("  ", indent)

	fmt.Printf("%s%s (%s)", indentStr, node.OriginalName, node.Type)

	if node.Script != "" {
		scriptPath := resolveResourcePath(node.Script, scene)
		if scriptPath != "" {
			fmt.Printf(" [Script: %s]", scriptPath)
		} else {
			fmt.Printf(" [Script: %s]", node.Script)
		}
	}

	fmt.Println()

	// Display properties
	if len(node.Properties) > 0 {
		if verbose {
			// Verbose mode: display all properties
			showAllProperties(node, indent+1, scene)
		} else {
			// Normal mode: display important properties only
			showImportantProperties(node, indent+1, scene)
		}
	}

	// Display child nodes recursively
	for _, child := range node.Children {
		printSceneTree(child, indent+1, scene)
	}
}

// showImportantProperties displays important properties
func showImportantProperties(node *GodotNode, indent int, scene *GodotScene) {
	indentStr := strings.Repeat("  ", indent)
	importantProps := []string{"position", "scale", "rotation", "size", "text", "texture", "visible"}

	for _, prop := range importantProps {
		if value, exists := node.Properties[prop]; exists {
			if prop == "texture" {
				// Resolve texture resource
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

// showAllProperties displays all properties (for verbose mode)
func showAllProperties(node *GodotNode, indent int, scene *GodotScene) {
	if len(node.Properties) == 0 {
		return
	}

	indentStr := strings.Repeat("  ", indent)

	for prop, value := range node.Properties {
		// Resolve resource references
		if strings.Contains(value, "ExtResource") || strings.Contains(value, "SubResource") {
			resolvedPath := resolveResourcePath(value, scene)
			if resolvedPath != "" {
				fmt.Printf("%s  %s: %s\n", indentStr, prop, resolvedPath)
				continue
			}
		}

		// Truncate values that are too long
		displayValue := value
		maxLen := 100
		if len(value) > maxLen {
			displayValue = value[:maxLen] + "..."
		}

		fmt.Printf("%s  %s: %s\n", indentStr, prop, displayValue)
	}
}

// resolveResourcePath resolves resource references to actual paths
func resolveResourcePath(resourceRef string, scene *GodotScene) string {
	// Parse ExtResource("1_abc123") format
	extResourceRe := regexp.MustCompile(`ExtResource\("([^"]*)"\)`)
	if matches := extResourceRe.FindStringSubmatch(resourceRef); len(matches) > 1 {
		resourceID := matches[1]
		if resource, exists := scene.ExtResources[resourceID]; exists {
			return resource.Path
		}
	}

	// Parse SubResource("SubResource_123") format
	subResourceRe := regexp.MustCompile(`SubResource\("([^"]*)"\)`)
	if matches := subResourceRe.FindStringSubmatch(resourceRef); len(matches) > 1 {
		resourceID := matches[1]
		if resource, exists := scene.SubResources[resourceID]; exists {
			return fmt.Sprintf("SubResource(%s)", resource.Type)
		}
	}

	return ""
}

// printSceneStats displays scene statistics
func printSceneStats(scene *GodotScene) {
	fmt.Println("=== Scene Statistics ===")
	fmt.Printf("Format Version: %d\n", scene.Format)
	fmt.Printf("Load Steps: %d\n", scene.LoadSteps)
	fmt.Printf("Total Nodes: %d\n", len(scene.AllNodes))
	fmt.Printf("Resources: %d\n", len(scene.Resources))

	// Count by node type
	typeCount := make(map[string]int)
	scriptCount := 0

	for _, node := range scene.AllNodes {
		typeCount[node.Type]++
		if node.Script != "" {
			scriptCount++
		}
	}

	fmt.Printf("Nodes with Scripts: %d\n", scriptCount)

	// Resource statistics
	fmt.Printf("ExtResources: %d\n", len(scene.ExtResources))
	fmt.Printf("SubResources: %d\n", len(scene.SubResources))

	fmt.Println("\nBy Node Type:")
	for nodeType, count := range typeCount {
		fmt.Printf("  %s: %d\n", nodeType, count)
	}

	// Count by ExtResource type
	if len(scene.ExtResources) > 0 {
		fmt.Println("\nBy ExtResource Type:")
		extTypeCount := make(map[string]int)
		for _, resource := range scene.ExtResources {
			extTypeCount[resource.Type]++
		}
		for extType, count := range extTypeCount {
			fmt.Printf("  %s: %d\n", extType, count)
		}
	}

	fmt.Println()
}

var rootCmd = &cobra.Command{
	Use:   "gdq [flags] <tscn file> [tscn files...]",
	Short: "Godot scene file parser",
	Long:  `Parse Godot .tscn files and display the scene tree structure.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Process first file
		tscnFile := args[0]

		// Check file existence
		if _, err := os.Stat(tscnFile); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", tscnFile)
		}

		// Parse tscn file
		scene, err := ParseTscnFile(tscnFile)
		if err != nil {
			return fmt.Errorf("parse error: %v", err)
		}

		// If node path is specified
		if nodePath != "" {
			targetNode := findNodeByPath(scene, nodePath)
			if targetNode == nil {
				return fmt.Errorf("node not found: %s", nodePath)
			}

			printNodeWithPath(scene, targetNode)
			return nil
		}

		// Display summary (optional)
		if showSummary {
			printSceneStats(scene)
		}

		// Display scene tree
		if scene.RootNode != nil {
			printSceneTree(scene.RootNode, 0, scene)
		} else {
			fmt.Println("Root node not found")
		}

		// Support multiple files
		if len(args) > 1 {
			for _, file := range args[1:] {
				// Check file existence
				if _, err := os.Stat(file); os.IsNotExist(err) {
					fmt.Printf("\nError: file not found: %s\n", file)
					continue
				}

				fmt.Printf("\n" + strings.Repeat("=", 50) + "\n")
				fmt.Printf("File: %s\n\n", file)

				scene, err := ParseTscnFile(file)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					continue
				}

				// If node path is specified
				if nodePath != "" {
					targetNode := findNodeByPath(scene, nodePath)
					if targetNode == nil {
						fmt.Printf("Error: node not found: %s\n", nodePath)
						continue
					}

					printNodeWithPath(scene, targetNode)
					continue
				}

				// Display summary (optional)
				if showSummary {
					printSceneStats(scene)
				}

				if scene.RootNode != nil {
					printSceneTree(scene.RootNode, 0, scene)
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&debugMode, "debug", "d", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&showSummary, "summary", "s", false, "Display statistics summary")
	rootCmd.Flags().StringVarP(&nodePath, "query", "q", "", "Search for a specific node path (e.g., \"Player/Sprite\")")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Display all properties in detail")
}

// Main function
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
