package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gavinmcnair/pictureprocess/pkg/imagedup"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Usage: %s <source_directory> <destination_directory>\n", filepath.Base(os.Args[0]))
	}

	sourceDir := os.Args[1]
	destDir := os.Args[2]

	numWorkers := runtime.NumCPU()
	err := imagedup.ProcessFiles(sourceDir, destDir, numWorkers)
	if err != nil {
		log.Fatalf("Failed to process files: %v", err)
	}

	fmt.Println("File processing complete")
}

