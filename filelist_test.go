package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFilePath_RejectsPathTraversalAttempts(t *testing.T) {
	root := t.TempDir()

	cases := []string{
		"../secret.txt",
		"../../etc/passwd",
		"folder/../../secret.txt",
		"..\\secret.txt",
	}
	for _, relPath := range cases {
		if _, ok := resolveFilePath(root, relPath); ok {
			t.Errorf("resolveFilePath(%q) should have been rejected as a traversal attempt", relPath)
		}
	}
}

func TestResolveFilePath_RejectsAbsolutePaths(t *testing.T) {
	root := t.TempDir()

	abs := filepath.Join(root, "..", "outside.txt")
	if _, ok := resolveFilePath(root, abs); ok {
		t.Errorf("resolveFilePath with an absolute path %q should have been rejected", abs)
	}
}

func TestResolveFilePath_AcceptsValidRelativePaths(t *testing.T) {
	root := t.TempDir()

	full, ok := resolveFilePath(root, "folder/book.txt")
	if !ok {
		t.Fatal("resolveFilePath should accept a plain relative path")
	}
	rootAbs, _ := filepath.Abs(root)
	wantAbs, _ := filepath.Abs(filepath.Join(rootAbs, "folder", "book.txt"))
	if full != wantAbs {
		t.Errorf("resolveFilePath = %q, want %q", full, wantAbs)
	}
}

func TestResolveFilePath_RejectsTheRootItself(t *testing.T) {
	root := t.TempDir()

	// "." (요청 경로가 비어있거나 정리 후 루트 자체를 가리키는 경우) — 파일 하나를 내려받는 엔드포인트라
	// 루트 폴더 자체를 대상으로 하는 요청은 유효하지 않다.
	if _, ok := resolveFilePath(root, "."); ok {
		t.Error("resolveFilePath(root, \".\") should be rejected")
	}
	if _, ok := resolveFilePath(root, ""); ok {
		t.Error("resolveFilePath(root, \"\") should be rejected")
	}
}

func TestListFilesRecursively_SkipsDotfilesAndDotDirectories(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "book.txt"), "content")
	mustWriteFile(t, filepath.Join(root, ".stfolder", "marker.txt"), "syncthing marker")
	mustMkdirAll(t, filepath.Join(root, ".hidden"))
	mustWriteFile(t, filepath.Join(root, ".hidden", "should-not-appear.txt"), "x")
	mustWriteFile(t, filepath.Join(root, ".DS_Store"), "x")

	files, err := listFilesRecursively(root)
	if err != nil {
		t.Fatalf("listFilesRecursively failed: %v", err)
	}

	if len(files) != 1 || files[0].RelativePath != "book.txt" {
		t.Errorf("expected only book.txt, got %+v", files)
	}
}

func TestListFilesRecursively_OnlyListsTxtAndZip(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "book.txt"), "a")
	mustWriteFile(t, filepath.Join(root, "archive.zip"), "b")
	mustWriteFile(t, filepath.Join(root, "cover.jpg"), "c")
	mustWriteFile(t, filepath.Join(root, "notes.md"), "d")

	files, err := listFilesRecursively(root)
	if err != nil {
		t.Fatalf("listFilesRecursively failed: %v", err)
	}

	got := map[string]bool{}
	for _, f := range files {
		got[f.RelativePath] = true
	}
	if len(got) != 2 || !got["book.txt"] || !got["archive.zip"] {
		t.Errorf("expected exactly {book.txt, archive.zip}, got %+v", got)
	}
}

func TestListFilesRecursively_UsesForwardSlashesForNestedPaths(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "series", "sub"))
	mustWriteFile(t, filepath.Join(root, "series", "sub", "chapter1.txt"), "a")

	files, err := listFilesRecursively(root)
	if err != nil {
		t.Fatalf("listFilesRecursively failed: %v", err)
	}

	if len(files) != 1 || files[0].RelativePath != "series/sub/chapter1.txt" {
		t.Errorf("expected series/sub/chapter1.txt, got %+v", files)
	}
}

func TestListFilesRecursively_ReturnsEmptySliceNotNilForAnEmptyFolder(t *testing.T) {
	root := t.TempDir()

	files, err := listFilesRecursively(root)
	if err != nil {
		t.Fatalf("listFilesRecursively failed: %v", err)
	}
	if files == nil {
		t.Error("expected an empty slice, got nil (this gets JSON-encoded as \"null\" instead of \"[]\")")
	}
	if len(files) != 0 {
		t.Errorf("expected no files, got %+v", files)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("failed to create dir %s: %v", path, err)
	}
}
