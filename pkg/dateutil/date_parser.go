package dateutil

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// dateLayouts defines formats to try parsing filename dates
var dateLayouts = []string{
	"2006-01-02", "02-01-2006", "2006/01/02",
	"02/01/2006", "20060102", "060102",
}

// ExtractDate uses EXIF and filename parsing to get an ISO date
func ExtractDate(filePath, filename string) (string, error) {
	// First, try to extract from EXIF data
	if date, err := extractExifDate(filePath); err == nil {
		return date, nil
	}

	// Else, parse date from file name
	if date, err := extractDateFromFilename(filename); err == nil {
		return date, nil
	}

	// Fallback: Use file's modification time
	return extractFileModTime(filePath)
}

// extractExifDate gets the date from EXIF data
func extractExifDate(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return "", err
	}

	date, err := x.DateTime()
	if err != nil {
		return "", err
	}

	return date.Format("2006-01-02"), nil
}

// extractDateFromFilename parses possible date formats
func extractDateFromFilename(filename string) (string, error) {
	re := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}|\d{2}/\d{2}/\d{4}|\d{8}|\d{6})`)
	match := re.FindStringSubmatch(filename)

	if len(match) > 0 {
		for _, layout := range dateLayouts {
			if t, err := time.Parse(layout, match[0]); err == nil {
				return t.Format("2006-01-02"), nil
			}
		}
	}

	return "", fmt.Errorf("no date found in filename")
}

// extractFileModTime provides modification time
func extractFileModTime(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	return info.ModTime().Format("2006-01-02"), nil
}

