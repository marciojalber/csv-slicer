package utils

import (
	"encoding/csv"
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
