package scheduler

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// StreamLog continuously streams lines from logPath to out.
// It polls every 100ms, detects file rotation or truncation,
// and gracefully reopens the active file. It terminates cleanly when ctx is canceled.
func StreamLog(ctx context.Context, logPath string, out io.Writer) error {
	var file *os.File
	var err error

	initialExists := false
	if _, err := os.Stat(logPath); err == nil {
		initialExists = true
	}

	// Wait until file exists or context is canceled
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		file, err = os.Open(logPath)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer func() {
		if file != nil {
			_ = file.Close()
		}
	}()

	// If the file already existed at invocation time, seek to end to stream only new lines.
	// If the file was created after invocation, stream from the beginning.
	if initialExists {
		_, _ = file.Seek(0, io.SeekEnd)
	}
	reader := bufio.NewReader(file)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line, err := reader.ReadString('\n')
			if err == nil {
				fmt.Fprint(out, line)
				continue
			}

			if err == io.EOF {
				if len(line) > 0 {
					fmt.Fprint(out, line)
				}

				// Check whether log was rotated or truncated
				curFi, statCurErr := file.Stat()
				newFi, statNewErr := os.Stat(logPath)
				if statCurErr == nil && statNewErr == nil {
					curPos, _ := file.Seek(0, io.SeekCurrent)
					// Rotated (different inode) or truncated (size decreased)
					if !os.SameFile(curFi, newFi) || newFi.Size() < curPos {
						_ = file.Close()
						newFile, openErr := os.Open(logPath)
						if openErr == nil {
							file = newFile
							reader = bufio.NewReader(file)
						}
					}
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(100 * time.Millisecond):
				}
				continue
			}

			return err
		}
	}
}
