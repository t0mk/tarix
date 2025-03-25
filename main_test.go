package main

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestEndToEnd tests the entire process of indexing, extracting, and verifying files from a TAR archive
func TestEndToEnd(t *testing.T) {
	// Step 1: Create a temporary directory
	dir, err := os.MkdirTemp("", "tar_test_dir")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(dir)

	// Step 2: Create some nonempty files in the temp directory
	fileContents := map[string]string{
		"file1.txt": "Hello, World!",
		"file2.txt": "This is a test.",
		"file3.txt": "Another file.",
	}

	for name, content := range fileContents {
		filePath := filepath.Join(dir, name)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write temp file: %v", err)
		}
	}

	// Step 3: Create a TAR file from the directory in another temp directory
	tarDir, err := os.MkdirTemp("", "tar_output")
	if err != nil {
		t.Fatalf("Failed to create temp directory for TAR: %v", err)
	}
	defer os.RemoveAll(tarDir)

	tarFilePath := filepath.Join(tarDir, "testarchive.tar")
	createTar(tarFilePath, dir)

	// Step 4: Create an index for the TAR file
	tarIndexPath := filepath.Join(tarDir, "testarchive.tar.index.json")
	if err := createTarIndex(tarFilePath, tarIndexPath); err != nil {
		t.Fatalf("Failed to create TAR index: %v", err)
	}

	// Step 5: Extract one file and verify contents
	extractDir, err := os.MkdirTemp("", "extract_test_dir")
	if err != nil {
		t.Fatalf("Failed to create temp extraction directory: %v", err)
	}
	defer os.RemoveAll(extractDir)

	extractedFilePath := filepath.Join(extractDir, "file1.txt")
	if err := extractFileFromTar(tarFilePath, tarIndexPath, "file1.txt", extractedFilePath); err != nil {
		t.Fatalf("Failed to extract file: %v", err)
	}

	// Verify that the extracted file content matches the original
	originalContent := fileContents["file1.txt"]
	extractedContent, err := os.ReadFile(extractedFilePath)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}

	if originalContent != string(extractedContent) {
		t.Errorf("Extracted content does not match. Expected: %s, Got: %s", originalContent, extractedContent)
	}
}

// createTar creates a tar file from the specified directory
func createTar(tarFilePath, dir string) error {
	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	tw := tar.NewWriter(tarFile)
	defer tw.Close()

	return filepath.Walk(dir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fi.IsDir() {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		header.Name = filepath.Base(file)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		if _, err := io.Copy(tw, bytes.NewReader(data)); err != nil {
			return err
		}

		return nil
	})
}