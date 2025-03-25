package main

import (
	"archive/tar"
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const HashLen = 16

var headerSize = int64(512)

func hashFilePath(filePath string) string {
	h := md5.New() // or use sha256.New() for stronger hashing
	h.Write([]byte(filePath))
	return hex.EncodeToString(h.Sum(nil))[:HashLen]
}

// createTarIndex creates an index for an existing TAR file
func createTarIndex(tarPath, indexPath string) error {
	// Open the TAR file
	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed to open tar file: %w", err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create a tar reader
	tr := tar.NewReader(file)

	// Create index
	index := TarIndex{
		Files: map[string]FileIndex{},
	}

	var currentPos int64 = 0
	var lastPercent int64 = -1

	// Iterate through the TAR archive
	for {
		headerPos := currentPos

		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar header: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			fileSize := header.Size
			paddedSize := (fileSize + 511) & ^int64(511)
			currentPos = headerPos + headerSize + paddedSize
			continue
		}

		cleanFilePath := filepath.Clean(header.Name)
		cleanFilePathHash := hashFilePath(cleanFilePath)

		fileIndex := FileIndex{
			Start: headerPos,
			Size:  header.Size,
		}

		if _, exists := index.Files[cleanFilePathHash]; exists {
			return fmt.Errorf("duplicate file path found for path %s: %s", cleanFilePath, cleanFilePathHash)
		}

		index.Files[cleanFilePathHash] = fileIndex

		paddedSize := (header.Size + 511) & ^int64(511)
		currentPos = headerPos + headerSize + paddedSize

		percentDone := (currentPos * 100) / fileInfo.Size()
		if percentDone != lastPercent {
			fmt.Printf("\rIndexing: %d%% complete", percentDone)
			lastPercent = percentDone
		}
	}

	// Open the output file for writing CSV
	outFile, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer outFile.Close()

	// Create a CSV writer
	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	// Write CSV header
	writer.Write([]string{"key", "start", "size"})

	// Write file entries to CSV
	for hsh, fileInfo := range index.Files {
		writer.Write([]string{
			hsh,
			fmt.Sprintf("%d", fileInfo.Start),
			fmt.Sprintf("%d", fileInfo.Size),
		})
	}

	fmt.Printf("\nCreated index with %d files\n", len(index.Files))
	fmt.Printf("Index saved to %s\n", indexPath)

	return nil
}

func ExtractBytesFromTarWithIndex(tindex *TarIndex, tarFile *os.File, filePath string) ([]byte, error) {

	// Replace cleanFilePath with its hash
	cleanFilePathHash := hashFilePath(filePath)

	// Find the file in the index using hash
	fileInfo, ok := tindex.Files[cleanFilePathHash]
	if !ok {
		return nil, fmt.Errorf("file %s not found in index", cleanFilePathHash)
	}

	// Seek to the file data position (after the header)
	dataPos := fileInfo.Start + headerSize
	if _, err := tarFile.Seek(dataPos, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to file position: %w", err)
	}

	// Read the file data
	data := make([]byte, fileInfo.Size)
	if _, err := io.ReadFull(tarFile, data); err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	return data, nil
}

// extractFileFromTar extracts a file from TAR using the index and writes it to a file
func extractFileFromTar(tarPath, indexPath, filePath, outputPath string) error {
	// Use the new function to read the index
	index, err := readTarIndex(indexPath)
	if err != nil {
		return err
	}

	// Open the TAR file
	tarFile, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed to open tar file: %w", err)
	}
	defer tarFile.Close()

	// Extract file data as bytes
	data, err := ExtractBytesFromTarWithIndex(index, tarFile, filePath)
	if err != nil {
		return err
	}

	// Write the data to the specified output
	var output io.Writer
	if outputPath == "-" {
		output = os.Stdout
	} else {
		outFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()
		output = outFile
	}

	if _, err := output.Write(data); err != nil {
		return fmt.Errorf("failed to write file data: %w", err)
	}

	if outputPath != "-" {
		fmt.Printf("Extracted %s to %s (size: %d bytes)\n", filePath, outputPath, len(data))
	}

	return nil
}

// listFilesInTar lists files in the TAR using the index
func listFilesInTar(indexPath string) error {
	// Use the new function to read the index
	index, err := readTarIndex(indexPath)
	if err != nil {
		return err
	}

	fmt.Printf("TAR archive contains %d files\n", len(index.Files))

	// Calculate total size of files
	var totalSize int64
	for _, fileInfo := range index.Files {
		totalSize += fileInfo.Size
	}

	fmt.Printf("Total content size: %d bytes\n\n", totalSize)
	fmt.Println("Files:")

	for hsh, fileInfo := range index.Files {
		// Format modification time for display
		fmt.Printf("- %s (%d bytes)\n", hsh, fileInfo.Size)
	}

	return nil
}

func readTarIndex(indexPath string) (*TarIndex, error) {
	// Open the index file
	file, err := os.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}
	defer file.Close()

	// Create a CSV reader
	reader := csv.NewReader(file)

	// Read and discard the header
	_, err = reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Initialize the index
	index := &TarIndex{
		Files: map[string]FileIndex{},
	}

	// Read each record from the CSV
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record: %w", err)
		}

		// Expecting the format: key, start, size
		if len(record) != 3 {
			return nil, fmt.Errorf("unexpected CSV format")
		}

		start, err := parseInt64(record[1])
		if err != nil {
			return nil, fmt.Errorf("invalid start value: %w", err)
		}

		size, err := parseInt64(record[2])
		if err != nil {
			return nil, fmt.Errorf("invalid size value: %w", err)
		}

		key := record[0]

		index.Files[key] = FileIndex{
			Start: start,
			Size:  size,
		}
	}

	return index, nil
}

func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func main() {
	// Command line flags for Index command
	indexCmd := flag.NewFlagSet("index", flag.ExitOnError)
	indexTarPath := indexCmd.String("tar", "", "TAR file to index")
	indexOutputPath := indexCmd.String("output", "", "Output index file (default: <tar>.index.json)")

	// Command line flags for Extract command
	extractCmd := flag.NewFlagSet("extract", flag.ExitOnError)
	extractTarPath := extractCmd.String("tar", "", "TAR file to extract from")
	extractIndexPath := extractCmd.String("index", "", "Index file for the TAR")
	extractFile := extractCmd.String("file", "", "File path to extract from the TAR")
	extractOutput := extractCmd.String("output", "", "Output file (default: extracted in current dir, '-' for stdout)")

	printfrompathCmd := flag.NewFlagSet("printfrompath", flag.ExitOnError)
	printfrompathTarPath := printfrompathCmd.String("tar", "", "TAR file to extract from")
	printfrompathIndexPath := printfrompathCmd.String("index", "", "Index file for the TAR")
	printfrompathFilePath := printfrompathCmd.String("file", "", "File path to extract from the TAR")

	// Command line flags for List command
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	listIndexPath := listCmd.String("index", "", "Index file to list")

	// Check if command line arguments were provided
	if len(os.Args) < 2 {
		fmt.Println("Expected 'index', 'extract', 'printfrompath' or 'list' command")
		fmt.Println("Usage:")
		fmt.Println("  index -tar <tar-file> -output <index-file>")
		fmt.Println("  extract -tar <tar-file> -index <index-file> -file <file-path> -output <output-file>")
		fmt.Println("  list -index <index-file>")
		fmt.Println("  printfrompath -tar <tar-file> -index <index-file> -file <file-path>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "index":
		indexCmd.Parse(os.Args[2:])
		if *indexTarPath == "" {
			fmt.Println("TAR file is required")
			indexCmd.PrintDefaults()
			os.Exit(1)
		}

		// Default output path if not specified
		outputPath := *indexOutputPath
		if outputPath == "" {
			outputPath = *indexTarPath + ".index.json"
		}

		err := createTarIndex(*indexTarPath, outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "printfrompath":
		printfrompathCmd.Parse(os.Args[2:])
		if *printfrompathTarPath == "" || *printfrompathIndexPath == "" || *printfrompathFilePath == "" {
			fmt.Println("TAR file, index file, and file to extract are required")
			printfrompathCmd.PrintDefaults()
			os.Exit(1)
		}

		tarIndex, err := readTarIndex(*printfrompathIndexPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		tarFile, err := os.Open(*printfrompathTarPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		bs, err := ExtractBytesFromTarWithIndex(tarIndex, tarFile, *printfrompathFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(string(bs))

	case "extract":
		extractCmd.Parse(os.Args[2:])
		if *extractTarPath == "" || *extractIndexPath == "" || *extractFile == "" {
			fmt.Println("TAR file, index file, and file to extract are required")
			extractCmd.PrintDefaults()
			os.Exit(1)
		}

		// Default output path if not specified
		outputPath := *extractOutput
		if outputPath == "" {
			outputPath = filepath.Base(*extractFile)
		}

		err := extractFileFromTar(*extractTarPath, *extractIndexPath, *extractFile, outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "list":
		listCmd.Parse(os.Args[2:])
		if *listIndexPath == "" {
			fmt.Println("Index file is required")
			listCmd.PrintDefaults()
			os.Exit(1)
		}

		err := listFilesInTar(*listIndexPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		fmt.Println("Expected 'index', 'extract', 'printfrompath' or 'list'")
		os.Exit(1)
	}
}
