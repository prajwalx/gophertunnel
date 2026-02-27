package transfer

import (
	"bufio"
	"context"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/prajwalx/gophertunnel/internal/logger"
	"github.com/schollz/progressbar/v3"
)

type DebugWriter struct {
	W     io.Writer
	Total int64
	Debug bool
}

func (dw *DebugWriter) Write(p []byte) (n int, err error) {
	n, err = dw.W.Write(p)
	dw.Total += int64(n)
	if dw.Debug {
		// Log every write operation
		logger.DebugLog(true, "NET-TX", "Chunk: %-6d | Cumulative: %d", n, dw.Total)
	}

	return n, err
}

type DebugReader struct {
	R     io.Reader
	Total int64
	Debug bool
}

func (dr *DebugReader) Read(p []byte) (n int, err error) {
	n, err = dr.R.Read(p)
	dr.Total += int64(n)
	if dr.Debug {
		// Log every read operation
		// fmt.Printf("[RECEIVER] Read chunk: %d bytes | Total: %d\n", n, dr.Total)
		logger.DebugLog(true, "NET-RX", "Chunk: %-6d | Cumulative: %d", n, dr.Total)
	}

	return n, err
}

func StreamFile(ctx context.Context, conn net.Conn, filePath string, stream cipher.Stream, debug bool) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	bar := progressbar.DefaultBytes(stat.Size(), "Sending")
	errChan := make(chan error, 1)

	go func() {
		// 1. Wrap the connection in a buffered writer to ensure smooth delivery
		bufWriter := bufio.NewWriter(conn)
		// Wrap the connection in a StreamWriter
		writer := &cipher.StreamWriter{S: stream, W: io.MultiWriter(bufWriter, bar)}

		debugW := &DebugWriter{W: writer, Debug: debug}
		size, err := io.Copy(debugW, file)
		fmt.Println("Bytes sent: ", size)
		if err != nil {
			errChan <- err
		}
		errChan <- bufWriter.Flush()

	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		if err == nil {

			// WAIT for the receiver to send a 1-byte "OK"
			// This blocks the Sender from exiting until the Receiver is actually done.
			ack := make([]byte, 1)
			conn.SetReadDeadline(time.Now().Add(time.Second * 60)) // 60s timeout
			_, err := conn.Read(ack)
			if err != nil || ack[0] != 1 {
				err2 := fmt.Errorf("receiver did not acknowledge final bytes: %v", err)
				fmt.Println(err2)
				return err2
			}

			fmt.Println("\n✅ Receiver confirmed receipt.")
			return nil
		}
		fmt.Println(err)
		return err
	}

}

func ReceiveFile(ctx context.Context, conn net.Conn, headerReader io.Reader, stream cipher.Stream, meta Metadata, debug bool) error {
	fileName := meta.FileName
	size := meta.Size
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	bar := progressbar.DefaultBytes(size, "Receiving")
	errChan := make(chan error, 1)

	go func() {
		// Setup hashing-on-the-fly
		hasher := sha256.New()

		// we know file size, so use limit reader
		limitReader := io.LimitReader(headerReader, meta.Size)
		reader := &cipher.StreamReader{S: stream, R: limitReader}

		// TeeReader updates progress bar while io.Copy pulls to File+Hasher
		// This ensures we hash while writing, saving an extra disk read!
		mw := io.MultiWriter(file, hasher)
		tr := io.TeeReader(reader, bar)

		debugR := &DebugReader{R: tr, Debug: debug}

		written, err := io.Copy(mw, debugR)
		fmt.Println("Bytes received ", written)
		if err != nil {
			errChan <- err
		}

		// Finalize and Validate Checksum
		file.Sync() // Force OS to write to physical disk
		finalHash := hex.EncodeToString(hasher.Sum(nil))

		if written == meta.Size && finalHash == meta.Checksum {
			conn.Write([]byte{1}) // Send Success ACK
			fmt.Printf("\n✅ Integrity Verified! %d bytes received.", written)
			errChan <- nil
		} else {
			conn.Write([]byte{0}) // Send Failure NACK
			errChan <- fmt.Errorf("\n❌ CORRUPTION: Expected %d bytes (%s), got %d bytes (%s)",
				meta.Size, meta.Checksum, written, finalHash)
		}

	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		if err == nil {

			fmt.Println("\n✅ File received and verified.")
			return nil
		}
		fmt.Println(err)
		return err
	}
}
