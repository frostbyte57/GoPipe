package wormhole

import (
	"archive/zip"
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

	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	mode := "file"
	if info.IsDir() {
		mode = "dir"
	}

	var reader io.Reader
	var size int64
	var name string

	// Handle Directory vs File
	if mode == "dir" {
		pr, pw := io.Pipe()
		reader = pr
		name = filepath.Base(filePath) + ".zip"
		size = 0 // Unknown size for stream (or we could calculate it, but keeping existing logic)

		// If we want to calculate size for progress bar, we need to walk first.
		// Existing logic walked it for size.
		filepath.Walk(filePath, func(_ string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				size += info.Size()
			}
			return nil
		})

		go func() {
			zw := zip.NewWriter(pw)
			baseDir := filepath.Dir(filePath) // parent
			// We ignore walk errors in the zip goroutine in existing code?
			// Existing code checks err in Walk.
			_ = filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				header, err := zip.FileInfoHeader(info)
				if err != nil {
					return err
				}
				relPath, _ := filepath.Rel(baseDir, path)
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
					f, err := os.Open(path)
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
			zw.Close()
			pw.Close()
		}()

	} else {
		f, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
		size = info.Size()
		name = info.Name()
	}

	// Send Metadata
	meta := transit.Metadata{
		Name: name,
		Size: size,
		Mode: mode,
	}
	metaBytes, _ := json.Marshal(meta)

	metaLen := uint32(len(metaBytes))
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, metaLen)

	if _, err := conn.Write(lenBuf); err != nil {
		return err
	}
	if _, err := conn.Write(metaBytes); err != nil {
		return err
	}

	// Transfer Data
	buf := make([]byte, 32*1024)
	var current int64

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			_, wErr := conn.Write(buf[:n])
			if wErr != nil {
				return wErr
			}
			current += int64(n)
			if size > 0 && progressCh != nil {
				ratio := float64(current) / float64(size)
				progressCh <- Progress{
					Current: current,
					Total:   size,
					Ratio:   ratio,
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
