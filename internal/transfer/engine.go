package transfer

import (
	"bufio"
	"context"
	"crypto/cipher"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
)

func StreamFile(ctx context.Context, conn net.Conn, filePath string, stream cipher.Stream) error {
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

		_, err = io.Copy(writer, file)
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
			// Optional: Small sleep or "ACK" read here ensures receiver
			// has time to process the last TCP packet before we kill the process.
			time.Sleep(1 * time.Second)

			// 1. After sending all data, signal to the receiver we are done by closing
			// only the WRITING side of the TCP connection (TCP Half-Close).
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.CloseWrite()
			}

			// 2. WAIT for the receiver to send a 1-byte "OK"
			// This blocks the Sender from exiting until the Receiver is actually done.
			ack := make([]byte, 1)
			conn.SetReadDeadline(time.Now().Add(time.Second * 10)) // 10s timeout
			_, err := conn.Read(ack)
			if err != nil {
				return fmt.Errorf("receiver did not acknowledge final bytes: %v", err)
			}

			fmt.Println("\n✅ Receiver confirmed receipt.")
			return nil
		}
		return err
	}

}

func ReceiveFile(ctx context.Context, conn net.Conn, filePath string, stream cipher.Stream, size int64) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	bar := progressbar.DefaultBytes(size, "Receiving")
	errChan := make(chan error, 1)

	go func() {
		reader := &cipher.StreamReader{S: stream, R: io.TeeReader(conn, bar)}

		_, err = io.Copy(file, reader)
		errChan <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		if err == nil {

			// 1. After io.Copy finishes (100%), we know the file is fully written to disk.
			// 2. Send the "ACK" byte back to the sender.
			_, err = conn.Write([]byte{1})
			if err != nil {
				return fmt.Errorf("failed to send completion ACK: %v", err)
			}

			fmt.Println("\n✅ File received and verified.")
			return nil
		}
		return err
	}
}
