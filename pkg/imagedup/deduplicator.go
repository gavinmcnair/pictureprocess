package imagedup

import (
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/corona10/goimagehash"
	"github.com/disintegration/imaging"
	"github.com/gavinmcnair/pictureprocess/pkg/dateutil"
)

// Supported image formats that we can natively process
var SupportedImageFormats = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// Supported RAW formats that need special handling
var SupportedRawFormats = map[string]bool{
	".nef": true, ".cr2": true, ".cr3": true,
	".arw": true, ".rw2": true, ".orf": true,
	".raf": true, ".pef": true, ".dng": true,
	".raw": true, ".kdc": true, ".sr2": true, ".mos": true, ".mrw": true,
}

var SupportedVideoFormats = map[string]bool{
	".avi":  true,
	".mp4":  true,
	".mkv":  true,
	".mov":  true,
}

type imageInfo struct {
	hash     uint64
	filename string
	isoDate  string
}

// ProcessFiles processes files, deduplicating by format requirements.
func ProcessFiles(srcDir, destDir string, numWorkers int) error {
	var fileList []string

	// Walk the directory recursively to collect files
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileList = append(fileList, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(fileList) == 0 {
		fmt.Println("No files found for processing.")
		return nil
	}

	var wg sync.WaitGroup
	fileChan := make(chan string, numWorkers)
	resultChan := make(chan imageInfo, len(fileList))
	var processedFiles uint64

	var imageCount, rawCount, videoCount, imageDuplicates, rawDuplicates, videoDuplicates, imageCopied, rawCopied, videoCopied uint64

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for file := range fileChan {
				ext := strings.ToLower(filepath.Ext(file))
				if SupportedImageFormats[ext] {
					atomic.AddUint64(&imageCount, 1)
					processImageFile(file, resultChan)
				} else if SupportedRawFormats[ext] {
					atomic.AddUint64(&rawCount, 1)
					processRawFile(file, resultChan)
				} else if SupportedVideoFormats[ext] {
					atomic.AddUint64(&videoCount, 1)
					processVideoFile(file, resultChan)
				} else {
					log.Printf("Unsupported file format: %s", file)
				}
				atomic.AddUint64(&processedFiles, 1)
				fmt.Printf("\rProcessing %d of %d files...", processedFiles, len(fileList))
			}
		}()
	}

	for _, fileName := range fileList {
		fileChan <- fileName
	}

	close(fileChan)
	wg.Wait()
	close(resultChan)

	fmt.Println("\nFiltering unique files...")

	uniqueFiles := filterUniqueFiles(resultChan)

	fmt.Println("Copying unique files...")

	dateCounters := make(map[string]uint64)

	for _, fileInfo := range uniqueFiles {
		dateStr := fileInfo.isoDate
		destPath := filepath.Join(destDir, dateStr)
		if err := os.MkdirAll(destPath, os.ModePerm); err != nil {
			log.Printf("Failed to create directory %s: %v", destPath, err)
			continue
		}

		relPath, err := filepath.Rel(srcDir, fileInfo.filename)
		if err != nil {
			log.Printf("Failed to compute relative path for %s: %v", fileInfo.filename, err)
			continue
		}

		dateCounters[dateStr]++
		newFileName := fmt.Sprintf("%03d%s", dateCounters[dateStr], filepath.Ext(fileInfo.filename))
		destFile := filepath.Join(destPath, newFileName)

		if err := copyFile(fileInfo.filename, destFile); err != nil {
			log.Printf("Failed to copy file to %s: %v", destFile, err)
			continue
		}

		// Create or update the index map for this directory
		mapping := map[string]string{relPath: newFileName}
		if err := writeIndexJSON(destPath, mapping); err != nil {
			log.Printf("Failed to write index.json in %s: %v", destPath, err)
			continue
		}

		// Increment copied counts
		if SupportedImageFormats[strings.ToLower(filepath.Ext(fileInfo.filename))] {
			atomic.AddUint64(&imageCopied, 1)
		} else if SupportedRawFormats[strings.ToLower(filepath.Ext(fileInfo.filename))] {
			atomic.AddUint64(&rawCopied, 1)
		} else if SupportedVideoFormats[strings.ToLower(filepath.Ext(fileInfo.filename))] {
			atomic.AddUint64(&videoCopied, 1)
		}
	}

	// Calculate duplicates
	imageDuplicates = imageCount - imageCopied
	rawDuplicates = rawCount - rawCopied
	videoDuplicates = videoCount - videoCopied

	// Print summary
	fmt.Printf("\nSummary:\n")
	fmt.Printf("%d images processed, %d duplicates found, %d copied\n", imageCount, imageDuplicates, imageCopied)
	fmt.Printf("%d RAW files processed, %d duplicates found, %d copied\n", rawCount, rawDuplicates, rawCopied)
	fmt.Printf("%d videos processed, %d duplicates found, %d copied\n", videoCount, videoDuplicates, videoCopied)

	fmt.Println("All files processed.")
	return nil
}

// writes the index.json file for each directory
func writeIndexJSON(destPath string, mapping map[string]string) error {
	indexFile := filepath.Join(destPath, "index.json")

	// Open index.json for reading and writing or create it if it doesn't exist
	f, err := os.OpenFile(indexFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Read existing data
	existingData := make(map[string]string)
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&existingData); err != nil && err != io.EOF {
		log.Printf("Error decoding existing JSON: %v", err)
		return err
	}

	// Update with new mappings
	for k, v := range mapping {
		existingData[k] = v
	}

	// Write the updated JSON map to the file
	f.Seek(0, 0) // Reset file pointer to the beginning
	f.Truncate(0) // Clear previous content
	encoder := json.NewEncoder(f)
	err = encoder.Encode(existingData)
	return err
}

// processFile handles the differentiation between image and other media processing.
func processFile(filePath string, resultChan chan<- imageInfo) {
	ext := strings.ToLower(filepath.Ext(filePath))

	if SupportedImageFormats[ext] {
		processImageFile(filePath, resultChan)
	} else if SupportedRawFormats[ext] {
		processRawFile(filePath, resultChan)
	} else if SupportedVideoFormats[ext] {
		processVideoFile(filePath, resultChan)
	} else {
		log.Printf("Skipping unsupported file format: %s", filePath)
	}
}

// processImageFile processes individual image files, computing hashes.
func processImageFile(filePath string, resultChan chan<- imageInfo) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Failed to open file: %s", filePath)
		return
	}
	defer file.Close()

	// Validate if it's an actual image file
	_, _, err = image.DecodeConfig(file)
	if err != nil {
		log.Printf("Skipping non-image or unsupported file: %s (%v)", filePath, err)
		return
	}

	file.Seek(0, 0) // Reset file read pointer

	img, err := imaging.Decode(file)
	if err != nil {
		log.Printf("Failed to decode file: %s", filePath)
		return
	}

	// Compute hash from the full image
	hash, err := goimagehash.AverageHash(img)
	if err != nil {
		log.Printf("Failed to compute hash: %s", filePath)
		return
	}

	date, err := dateutil.ExtractDate(filePath, filepath.Base(filePath))
	if err != nil {
		log.Printf("Failed to extract date: %s", filePath)
		return
	}

	resultChan <- imageInfo{
		hash:     hash.GetHash(),
		filename: filePath,
		isoDate:  date,
	}
}

// processRawFile handles RAW image formats similarly to video processing.
func processRawFile(filePath string, resultChan chan<- imageInfo) {
	info, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Failed to get fileinfo: %s", filePath)
		return
	}
	fileSize := info.Size()

	// Use file size as a trivial comparison point for hash
	hash := uint64(fileSize)

	date, err := dateutil.ExtractDate(filePath, filepath.Base(filePath))
	if err != nil {
		log.Printf("Failed to extract date: %s", filePath)
		if dateTime, err := extractFileCreationDate(filePath); err == nil {
			date = dateTime
		}
	}

	resultChan <- imageInfo{
		hash:     hash,
		filename: filePath,
		isoDate:  date,
	}
}

// processVideoFile processes individual video files deduplicated on size and name.
func processVideoFile(filePath string, resultChan chan<- imageInfo) {
	info, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Failed to get fileinfo: %s", filePath)
		return
	}
	fileSize := info.Size()

	// Use file size as a trivial comparison point for hash
	hash := uint64(fileSize)

	date, err := dateutil.ExtractDate(filePath, filepath.Base(filePath))
	if err != nil {
		log.Printf("Failed to extract date: %s", filePath)
		if dateTime, err := extractFileCreationDate(filePath); err == nil {
			date = dateTime
		}
	}

	resultChan <- imageInfo{
		hash:     hash,
		filename: filePath,
		isoDate:  date,
	}
}

// extractFileCreationDate retrieves the metadata for file creation date
func extractFileCreationDate(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	// Retrieve modification time as a best-effort representation of creation
	modTime := info.ModTime().Format("2006-01-02")
	return modTime, nil
}

// filterUniqueFiles retains only the largest file with the same hash
func filterUniqueFiles(files chan imageInfo) map[uint64]imageInfo {
	unique := make(map[uint64]imageInfo)
	hashSizes := make(map[uint64]int64)

	for fileInfo := range files {
		info, _ := os.Stat(fileInfo.filename)
		fileSize := info.Size()
		if _, exists := unique[fileInfo.hash]; !exists || fileSize > hashSizes[fileInfo.hash] {
			unique[fileInfo.hash] = fileInfo
			hashSizes[fileInfo.hash] = fileSize
		}
	}

	return unique
}

// copyFile copies a file from source to destination path, preserving binary content.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return nil
}

