package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Check if submodule is initialized
func checkSubmoduleInitialized(t *testing.T) bool {
	demoProjectsPath := "test/godot-demo-projects"
	if _, err := os.Stat(demoProjectsPath); os.IsNotExist(err) {
		t.Skip("godot-demo-projects submodule not initialized. Please run 'git submodule update --init'.")
		return false
	}
	return true
}

// Find all tscn files in godot-demo-projects
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

// Parse all tscn files in demo projects
func TestGodotDemoProjects(t *testing.T) {
	if !checkSubmoduleInitialized(t) {
		return
	}

	demoProjectsPath := "test/godot-demo-projects"

	t.Logf("Searching demo project directory: %s", demoProjectsPath)

	tscnFiles, err := findTscnFiles(demoProjectsPath)
	if err != nil {
		t.Fatalf("tscn file search error: %v", err)
	}

	if len(tscnFiles) == 0 {
		t.Fatal("No tscn files found")
	}

	t.Logf("Detected tscn files: %d", len(tscnFiles))

	successCount := 0
	failCount := 0
	var failedFiles []string

	for _, file := range tscnFiles {
		t.Run(file, func(t *testing.T) {
			scene, err := ParseTscnFile(file)
			if err != nil {
				failCount++
				failedFiles = append(failedFiles, file)
				t.Errorf("Parse error: %v", err)
				return
			}

			// Basic validation check
			if scene == nil {
				failCount++
				failedFiles = append(failedFiles, file)
				t.Error("Scene is nil")
				return
			}

			// Ensure at least one node exists
			if len(scene.AllNodes) == 0 {
				t.Logf("Warning: No nodes found (possibly empty scene)")
			}

			successCount++
		})
	}

	// Display summary
	t.Logf("\n=== Test Results Summary ===")
	t.Logf("Total files: %d", len(tscnFiles))
	t.Logf("Success: %d", successCount)
	t.Logf("Failed: %d", failCount)

	if len(failedFiles) > 0 {
		t.Logf("\nFailed files:")
		for _, file := range failedFiles {
			t.Logf("  - %s", file)
		}
	}

	// Check success rate
	successRate := float64(successCount) / float64(len(tscnFiles)) * 100
	t.Logf("Success rate: %.2f%%", successRate)

	// Warn if success rate is below 80%
	if successRate < 80.0 {
		t.Errorf("Success rate is too low (%.2f%% < 80%%)", successRate)
	}
}

// Test specific demo projects in detail
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
			// Check file existence
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skipf("File not found: %s", tc.path)
				return
			}

			scene, err := ParseTscnFile(tc.path)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			// Check node count
			if len(scene.AllNodes) < tc.minNodes {
				t.Errorf("Node count less than expected (expected: %d or more, got: %d)",
					tc.minNodes, len(scene.AllNodes))
			}

			// Check root node
			if tc.shouldHaveRoot && scene.RootNode == nil {
				t.Error("Root node not found")
			}

			if scene.RootNode != nil {
				t.Logf("Root node: %s (%s)", scene.RootNode.OriginalName, scene.RootNode.Type)
				t.Logf("Total nodes: %d", len(scene.AllNodes))
				t.Logf("Child nodes: %d", len(scene.RootNode.Children))
			}
		})
	}
}

// Performance test (optional)
func TestParsingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test (-short flag)")
	}

	if !checkSubmoduleInitialized(t) {
		return
	}

	demoProjectsPath := "test/godot-demo-projects"
	tscnFiles, err := findTscnFiles(demoProjectsPath)
	if err != nil {
		t.Fatalf("tscn file search error: %v", err)
	}

	if len(tscnFiles) == 0 {
		t.Skip("No tscn files found")
	}

	// Performance test with first 10 files
	testFiles := tscnFiles
	if len(testFiles) > 10 {
		testFiles = tscnFiles[:10]
	}

	for _, file := range testFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			_, err := ParseTscnFile(file)
			if err != nil {
				t.Logf("Parse error (continuing for performance test): %v", err)
			}
		})
	}
}
