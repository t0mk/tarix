package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/t0mk/tarix"
)

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

		err := tarix.CreateTarIndex(*indexTarPath, outputPath)
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

		tarixHandle, err := tarix.NewTarixHandle(*printfrompathTarPath, *printfrompathIndexPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer tarixHandle.TarFile.Close()

		// Extract file data as bytes
		bs, err := tarixHandle.ExtractBytesOfFile(*printfrompathFilePath)
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

		err := tarix.ExtractFileFromTar(*extractTarPath, *extractIndexPath, *extractFile, outputPath)
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

		err := tarix.ListFilesInTar(*listIndexPath)
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
