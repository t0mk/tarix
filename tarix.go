package tarix

import (
	"archive/tar"
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
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

// CreateTarIndex creates an index for an existing TAR file
func CreateTarIndex(tarPath, indexPath string) error {
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

type TarixHandle struct {
	TarFile *os.File
	Index   *TarIndex
}

func NewTarixHandle(tarPath, indexPath string) (*TarixHandle, error) {
	index, err := ReadTarIndex(indexPath)
	if err != nil {
		return nil, err
	}

	tarFile, err := os.Open(tarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tar file: %w", err)
	}
	return &TarixHandle{
		TarFile: tarFile,
		Index:   index,
	}, nil
}

func (th *TarixHandle) ExtractBytesOfFile(filePath string) ([]byte, error) {
	// Replace cleanFilePath with its hash
	cleanFilePathHash := hashFilePath(filePath)

	// Find the file in the index using hash
	fileInfo, ok := th.Index.Files[cleanFilePathHash]
	if !ok {
		return nil, fmt.Errorf("file %s not found in index", cleanFilePathHash)
	}

	// Seek to the file data position (after the header)
	dataPos := fileInfo.Start + headerSize
	if _, err := th.TarFile.Seek(dataPos, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to file position: %w", err)
	}

	// Read the file data
	data := make([]byte, fileInfo.Size)
	if _, err := io.ReadFull(th.TarFile, data); err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}
	return data, nil

}

// ExtractFileFromTar extracts a file from TAR using the index and writes it to a file
func ExtractFileFromTar(tarPath, indexPath, filePath, outputPath string) error {
	tarixHandle, err := NewTarixHandle(tarPath, indexPath)
	if err != nil {
		return err
	}
	defer tarixHandle.TarFile.Close()

	// Extract file data as bytes
	data, err := tarixHandle.ExtractBytesOfFile(filePath)
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

// ListFilesInTar lists files in the TAR using the index
func ListFilesInTar(indexPath string) error {
	// Use the new function to read the index
	index, err := ReadTarIndex(indexPath)
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

func ReadTarIndex(indexPath string) (*TarIndex, error) {
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
