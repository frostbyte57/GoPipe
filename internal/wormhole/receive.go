package wormhole

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/frostbyte57/GoPipe/internal/transit"
)

type Progress struct {
	Current int64
	Total   int64
	Ratio   float64
}

// ReceiveFile receives a file from the sender.
// It tracks progress via the provided channel.
func (c *Client) ReceiveFile(ctx context.Context, outDir string, progressCh chan<- Progress) (string, error) {
	conn, err := c.PerformTransfer(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// Read Metadata Length
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return "", err
	}
	metaLen := binary.BigEndian.Uint32(lenBuf)

	// Read Metadata
	metaBuf := make([]byte, metaLen)
	if _, err := io.ReadFull(conn, metaBuf); err != nil {
		return "", err
	}

	var meta transit.Metadata
	if err := json.Unmarshal(metaBuf, &meta); err != nil {
		return "", err
	}

	// Determine Output Path
	if outDir == "" {
		outDir = "."
	}
	outPath := filepath.Join(outDir, meta.Name)

	// Auto-rename if exists
	ext := filepath.Ext(meta.Name)
	nameOnly := meta.Name[:len(meta.Name)-len(ext)]
	counter := 1
	for {
		if _, err := os.Stat(outPath); os.IsNotExist(err) {
			break
		}
		outPath = filepath.Join(outDir, fmt.Sprintf("%s (%d)%s", nameOnly, counter, ext))
		counter++
	}

	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	outWriter := bufio.NewWriterSize(out, 64*1024*1024)
	defer outWriter.Flush()

	var received int64
	buf := make([]byte, 1024*1024)
	for {
		n, rErr := conn.Read(buf)
		if n > 0 {
			_, wErr := outWriter.Write(buf[:n])
			if wErr != nil {
				return "", wErr
			}
			received += int64(n)
			if meta.Size > 0 && progressCh != nil {
				ratio := float64(received) / float64(meta.Size)
				progressCh <- Progress{
					Current: received,
					Total:   meta.Size,
					Ratio:   ratio,
				}
			}
		}
		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			return "", rErr
		}
	}

	return filepath.Base(outPath), nil
}
