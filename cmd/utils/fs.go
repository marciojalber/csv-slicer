// code/utils/fs.go

package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
)

func OpenFileOrError(filename string, err_msg string) *os.File {
	file, err := os.Open(filename)

	if err != nil {
		log.Fatal(err_msg)
	}

	return file
}

func NewCsvReader(file *os.File, separator rune) *csv.Reader {
	// info, _ := file.Stat()
	// offset := reader.InputOffset()
	// percent := float64(offset) / float64(info.Size())

	reader := csv.NewReader(file)
	reader.Comma = separator
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // allow variable fields

	return reader
}

func ReadCsvLine(reader *csv.Reader) ([]string, bool) {
	record, err := reader.Read()

	if err == io.EOF {
		return []string{}, true
	}

	if err != nil {
		log.Fatal(err)
	}

	return record, false
}

func FormatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
		PB = TB * 1024
	)

	switch {
	case size >= PB:
		return fmt.Sprintf("%.2f PB", float64(size)/PB)
	case size >= TB:
		return fmt.Sprintf("%.2f TB", float64(size)/TB)
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}
