package client

import (
	"bytes"
	"compress/gzip"
	"io"
)

type CompressionType uint8

const (
	CompressionNone CompressionType = 0
	CompressionGzip CompressionType = 1
	CompressionZlib CompressionType = 2
)

type CompressedData struct {
	Type CompressionType
	Data []byte
}

func CompressGzip(data []byte) *CompressedData {
	var buf bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	writer.Write(data)
	writer.Close()
	return &CompressedData{Type: CompressionGzip, Data: buf.Bytes()}
}

func Decompress(data []byte) []byte {
	if len(data) < 2 {
		return data
	}

	compressionType := CompressionType(data[0])
	if compressionType == CompressionNone {
		return data[1:]
	}

	compressed := data[1:]

	switch compressionType {
	case CompressionGzip:
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return compressed
		}
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return compressed
		}
		return decompressed
	}

	return compressed
}

func ShouldCompress(dataLen, threshold int) bool {
	return dataLen >= threshold
}