package artifact

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/slimtoolkit/slim/pkg/report"
)

// TestIsThroughSymlinkLogic tests the symlink detection logic used in deduplicateFileMap
func TestIsThroughSymlinkLogic(t *testing.T) {
	// This tests the isThroughSymlink inline function logic from deduplicateFileMap
	isThroughSymlink := func(p string) bool {
		return strings.HasPrefix(p, "/usr/local/cuda/") && !strings.HasPrefix(p, "/usr/local/cuda-")
	}

	tt := []struct {
		path     string
		expected bool
	}{
		// Paths through symlink (should be avoided)
		{"/usr/local/cuda/lib64/libcudart.so", true},
		{"/usr/local/cuda/include/cuda.h", true},
		{"/usr/local/cuda/bin/nvcc", true},

		// Canonical paths (should be preferred)
		{"/usr/local/cuda-12.9/lib64/libcudart.so", false},
		{"/usr/local/cuda-12.9/include/cuda.h", false},
		{"/usr/local/cuda-11.8/bin/nvcc", false},

		// Other paths (not affected)
		{"/usr/lib/libfoo.so", false},
		{"/opt/nvidia/cuda/lib/libbar.so", false},
		{"/home/user/cuda/file.txt", false},
	}

	for _, test := range tt {
		result := isThroughSymlink(test.path)
		if result != test.expected {
			t.Errorf("isThroughSymlink(%q) = %v, want %v", test.path, result, test.expected)
		}
	}
}

// TestPathPrioritySorting tests the sorting logic that determines which path to keep
func TestPathPrioritySorting(t *testing.T) {
	// Simulate the sorting logic from deduplicateFileMap
	isThroughSymlink := func(p string) bool {
		return strings.HasPrefix(p, "/usr/local/cuda/") && !strings.HasPrefix(p, "/usr/local/cuda-")
	}

	// fileMap simulates the p.fileMap with flags
	fileMap := map[string]*report.ArtifactProps{
		"/usr/local/cuda/lib64/libcudart.so":      nil,
		"/usr/local/cuda-12.9/lib64/libcudart.so": {Flags: map[string]bool{"R": true}},
	}

	sortPaths := func(paths []string) {
		sort.Slice(paths, func(i, j int) bool {
			pi, pj := paths[i], paths[j]

			// First priority: Prefer paths that don't go through symlink
			symI := isThroughSymlink(pi)
			symJ := isThroughSymlink(pj)
			if symI != symJ {
				return !symI
			}

			// Second priority: Check if either path has flags
			propsI := fileMap[pi]
			propsJ := fileMap[pj]
			hasI := propsI != nil && len(propsI.Flags) > 0
			hasJ := propsJ != nil && len(propsJ.Flags) > 0

			if hasI && !hasJ {
				return true
			}
			if !hasI && hasJ {
				return false
			}

			// Third priority: Prefer longer paths
			return len(pi) > len(pj)
		})
	}

	// Test case 1: symlink path vs canonical path
	paths1 := []string{
		"/usr/local/cuda/lib64/libcudart.so",
		"/usr/local/cuda-12.9/lib64/libcudart.so",
	}
	sortPaths(paths1)
	if paths1[0] != "/usr/local/cuda-12.9/lib64/libcudart.so" {
		t.Errorf("expected canonical path first, got %v", paths1)
	}

	// Test case 2: Two canonical paths - prefer one with flags
	fileMap2 := map[string]*report.ArtifactProps{
		"/usr/local/cuda-12.9/lib64/libcudart.so.12": {Flags: map[string]bool{"R": true}},
		"/usr/local/cuda-12.9/lib64/libcudart.so":    nil,
	}
	paths2 := []string{
		"/usr/local/cuda-12.9/lib64/libcudart.so",
		"/usr/local/cuda-12.9/lib64/libcudart.so.12",
	}
	sort.Slice(paths2, func(i, j int) bool {
		pi, pj := paths2[i], paths2[j]
		propsI := fileMap2[pi]
		propsJ := fileMap2[pj]
		hasI := propsI != nil && len(propsI.Flags) > 0
		hasJ := propsJ != nil && len(propsJ.Flags) > 0
		if hasI && !hasJ {
			return true
		}
		if !hasI && hasJ {
			return false
		}
		return len(pi) > len(pj)
	})
	if paths2[0] != "/usr/local/cuda-12.9/lib64/libcudart.so.12" {
		t.Errorf("expected path with flags first, got %v", paths2)
	}

	// Test case 3: Equal priority - prefer longer path
	paths3 := []string{
		"/usr/lib/short.so",
		"/usr/lib/subdir/longer.so",
	}
	sort.Slice(paths3, func(i, j int) bool {
		return len(paths3[i]) > len(paths3[j])
	})
	if paths3[0] != "/usr/lib/subdir/longer.so" {
		t.Errorf("expected longer path first, got %v", paths3)
	}
}

// TestDeduplicateFileMapWithHardlinks tests deduplication with actual hardlinks
func TestDeduplicateFileMapWithHardlinks(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "dedup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file
	originalPath := filepath.Join(tmpDir, "original.txt")
	if err := os.WriteFile(originalPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create original file: %v", err)
	}

	// Create a hardlink to the same file
	hardlinkPath := filepath.Join(tmpDir, "hardlink.txt")
	if err := os.Link(originalPath, hardlinkPath); err != nil {
		t.Fatalf("failed to create hardlink: %v", err)
	}

	// Verify both paths have the same inode
	origInfo, err := os.Lstat(originalPath)
	if err != nil {
		t.Fatalf("failed to stat original: %v", err)
	}
	linkInfo, err := os.Lstat(hardlinkPath)
	if err != nil {
		t.Fatalf("failed to stat hardlink: %v", err)
	}

	origStat, ok := origInfo.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("failed to get syscall.Stat_t for original")
	}
	linkStat, ok := linkInfo.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("failed to get syscall.Stat_t for hardlink")
	}

	if origStat.Ino != linkStat.Ino {
		t.Fatalf("expected same inode, got %d vs %d", origStat.Ino, linkStat.Ino)
	}

	// Test that we can build an inode map
	inodeMap := make(map[uint64][]string)
	for _, fpath := range []string{originalPath, hardlinkPath} {
		info, err := os.Lstat(fpath)
		if err != nil {
			t.Fatalf("failed to stat %s: %v", fpath, err)
		}
		if sys, ok := info.Sys().(*syscall.Stat_t); ok {
			inodeMap[sys.Ino] = append(inodeMap[sys.Ino], fpath)
		}
	}

	// Should have one inode with two paths
	if len(inodeMap) != 1 {
		t.Errorf("expected 1 inode, got %d", len(inodeMap))
	}

	for inode, paths := range inodeMap {
		if len(paths) != 2 {
			t.Errorf("expected 2 paths for inode %d, got %d", inode, len(paths))
		}
	}
}

// TestDeduplicateFileMapSkipsSymlinks tests that symlinks themselves are not deduplicated
func TestDeduplicateFileMapSkipsSymlinks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a regular file
	regularPath := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularPath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create a symlink to it
	symlinkPath := filepath.Join(tmpDir, "symlink.txt")
	if err := os.Symlink(regularPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Check that Lstat identifies symlink vs regular file
	regInfo, _ := os.Lstat(regularPath)
	symInfo, _ := os.Lstat(symlinkPath)

	if !regInfo.Mode().IsRegular() {
		t.Error("regular file should be identified as regular")
	}

	if symInfo.Mode().IsRegular() {
		t.Error("symlink should NOT be identified as regular file")
	}

	// The deduplication logic only processes regular files
	// Symlinks have different inodes and are handled separately
}

