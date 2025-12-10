package wormhole

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/frostbyte57/GoPipe/internal/transit"
)

// SendFile sends a file or directory to the receiver.
// It tracks progress via the provided channel.
func (c *Client) SendFile(ctx context.Context, filePath string, progressCh chan<- Progress) error {
	conn, err := c.PerformTransfer(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	reader, size, name, mode, err := prepareStream(filePath)
	if err != nil {
		return err
	}
	// reader is responsible for closing underlying files if any, but io.PipeReader doesn't need explicit close if the writer closes,
	// however os.File does.
	// We need to handle cleanup.
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}

	// Send Metadata
	meta := transit.Metadata{
		Name: name,
		Size: size,
		Mode: mode,
	}
	metaBytes, _ := json.Marshal(meta)

	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(metaBytes)))

	if _, err := conn.Write(lenBuf); err != nil {
		return err
	}
	if _, err := conn.Write(metaBytes); err != nil {
		return err
	}

	// Transfer Data
	bufReader := bufio.NewReaderSize(reader, 64*1024*1024)
	buf := make([]byte, 1024*1024)
	var current int64

	for {
		n, err := bufReader.Read(buf)
		if n > 0 {
			if _, wErr := conn.Write(buf[:n]); wErr != nil {
				return wErr
			}
			current += int64(n)
			if size > 0 && progressCh != nil {
				progressCh <- Progress{
					Current: current,
					Total:   size,
					Ratio:   float64(current) / float64(size),
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func prepareStream(path string) (io.Reader, int64, string, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, "", "", err
	}

	if info.IsDir() {
		return prepareDirectoryStream(path)
	}
	return prepareFileStream(path, info)
}

func prepareFileStream(path string, info os.FileInfo) (io.Reader, int64, string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, "", "", err
	}
	return f, info.Size(), info.Name(), "file", nil
}

func prepareDirectoryStream(path string) (io.Reader, int64, string, string, error) {
	pr, pw := io.Pipe()
	name := filepath.Base(path) + ".zip"
	var size int64

	// Calculate total size for progress
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	go func() {
		defer pw.Close()
		zw := zip.NewWriter(pw)
		defer zw.Close()

		baseDir := filepath.Dir(path)
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}

			relPath, _ := filepath.Rel(baseDir, filePath)
			header.Name = relPath

			if info.IsDir() {
				header.Name += "/"
			} else {
				header.Method = zip.Deflate
			}

			w, err := zw.CreateHeader(header)
			if err != nil {
				return err
			}

			if !info.IsDir() {
				f, err := os.Open(filePath)
				if err != nil {
					return err
				}
				defer f.Close()
				if _, err := io.Copy(w, f); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			// In a real app we might want to propagate this error to the reader?
			// The PipeWriter.CloseWithError(err) could be used here.
			pw.CloseWithError(err)
		}
	}()

	return pr, size, name, "dir", nil
}
