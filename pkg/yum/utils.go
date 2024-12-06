package yum

import (
	"bufio"
	"io"

	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
)

// Converts any struct to a pointer to that struct
func Ptr[T any](item T) *T {
	return &item
}

func ExtractIfCompressed(reader io.ReadCloser) (extractedReader io.Reader, err error) {
	bufferedReader := bufio.NewReader(reader)
	header, err := bufferedReader.Peek(20)
	if err != nil {
		return nil, err
	}
	fileType, err := filetype.Match(header)
	if err != nil {
		return nil, err
	}

	// handle compressed file
	if fileType == matchers.TypeGz || fileType == matchers.TypeZstd || fileType == matchers.TypeXz {
		extractedReader, err = ParseCompressedData(bufferedReader)
		if err != nil {
			return nil, err
		}
		return extractedReader, nil
	} else {
		// handle uncompressed comps
		return bufferedReader, nil
	}
}
