package yum

import (
	"bufio"
	"fmt"
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

// CheckLimit inspects an io.Reader (typically returned from io.LimitReader)
// to see if the byte limit has been exceeded.
func CheckLimit(r io.Reader, maxSize int64) error {
	if lr, ok := r.(*io.LimitedReader); ok && lr.N == 0 {
		return fmt.Errorf("decompression limit of %d bytes exceeded", maxSize)
	}
	return nil
}
