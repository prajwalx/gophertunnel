package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/prajwalx/gophertunnel/internal/discovery"
	"github.com/prajwalx/gophertunnel/internal/logger"
	"github.com/prajwalx/gophertunnel/internal/security"
	"github.com/prajwalx/gophertunnel/internal/transfer"
	"github.com/spf13/cobra"
)

type LoggingConn struct {
	net.Conn
	debug bool
}

func (l LoggingConn) Write(b []byte) (int, error) {
	n, err := l.Conn.Write(b)
	if l.debug {
		logger.DebugLog(true, "TCP-TX", "Bytes: %d | Err: %v\n", n, err)
	}

	return n, err
}

func (l LoggingConn) Read(b []byte) (int, error) {
	n, err := l.Conn.Read(b)
	if l.debug {
		logger.DebugLog(true, "TCP-RX", "Bytes: %d | Err: %v\n", n, err)
	}

	return n, err
}

// fileExists checks if a file exists and is not a directory.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	// Check if it's a file, not a directory
	return err == nil && !info.IsDir()
}

func setUpLogger(debug bool) {
	if debug {
		// Ldate: 2026/02/27, Ltime: 18:43:06, Lmicroseconds: .123456
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lmsgprefix)
		log.SetPrefix("[GopherTunnel] ")
	}
}

func main() {
	var debugMode bool
	const portStr = ":8080"

	// context to stop go routines on os interrupt
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var rootCmd = &cobra.Command{Use: "tunnel"}

	var sendCmd = &cobra.Command{Use: "send [file]",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// file exist
			// open tunnel
			// wait
			setUpLogger(debugMode)

			path := args[0]

			if !fileExists(path) {
				fmt.Println("File do not exist in given path ", path)
				os.Exit(1)
			}

			const port = 8080

			//start mdns in background
			go discovery.StartServer(ctx, port)

			// start tcp server
			ln, err := net.Listen("tcp", portStr)
			if err != nil {
				fmt.Println("Unable to open tcp server on port ", portStr)
				os.Exit(1)
			}

			fmt.Printf("📦 Peer discovery active. Waiting for receiver...\n")
			fmt.Println("Waiting for receiver on ", portStr, " ...")

			connChan := make(chan net.Conn, 1)

			// go routine to wait for a connection accept from the receiver
			go func() {
				conn, errCon := ln.Accept()
				if errCon != nil {
					fmt.Println("Unable to accept for receiver on ", portStr, " ...")
					os.Exit(1)
				}
				loggedConn := LoggingConn{conn, debugMode}
				connChan <- loggedConn
			}()

			select {
			case <-ctx.Done():
				return
			case conn := <-connChan:
				defer conn.Close()
				fileInfo, err := os.Stat(path)

				if err != nil {
					fmt.Println("Error getting file info")
					os.Exit(1)
				}

				fmt.Print("🔍 Calculating SHA-256 Checksum... ")
				hash, err := transfer.CalculateFileHash(path)
				if err != nil {
					fmt.Println("Error calculating file hash")
					os.Exit(1)
				}

				// send filename, size and checksum in the header
				transfer.SendHeader(conn, transfer.Metadata{FileName: fileInfo.Name(), Size: fileInfo.Size(), Checksum: hash})

				stream, err := security.GetStream(security.SharedKey)

				if err != nil {
					fmt.Println("Error getting cipher stream")
					os.Exit(1)
				}

				transfer.StreamFile(ctx, conn, path, stream, debugMode)
				logger.DebugLog(debugMode, "SENDER", "Returning main.go send cmd")

			}

		},
	}

	var receiveCmd = &cobra.Command{Use: "receive",
		Run: func(cmd *cobra.Command, args []string) {

			setUpLogger(debugMode)

			// to cancel mdn discovery when a peer is found
			discoverCtx, cancelDiscover := context.WithCancel(ctx)

			ip, err := discovery.FindPeer(discoverCtx)

			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			} else {
				fmt.Println("IP: ", ip)
			}

			cancelDiscover()

			connn, err := net.Dial("tcp", ip+portStr)
			conn := LoggingConn{connn, debugMode}
			if err != nil {
				fmt.Println("Unable to dial to tcp server on ", ip, portStr)
				os.Exit(1)
			}

			defer conn.Close()

			// read tcp header to get metadata
			reader := bufio.NewReader(conn)
			headerLine, err := reader.ReadBytes('\n')

			if err != nil {
				fmt.Println("Unable to read header ")
				os.Exit(1)
			}
			var meta transfer.Metadata
			err = json.Unmarshal(headerLine, &meta)

			if err != nil {
				fmt.Println("Unable to unmarshal json ")
				os.Exit(1)
			}

			stream, err := security.GetStream(security.SharedKey)

			if err != nil {
				fmt.Println("Unable to get cipher stream")
				os.Exit(1)
			}

			transfer.ReceiveFile(ctx, conn, reader, stream, meta, debugMode)
			logger.DebugLog(debugMode, "RECEIVER", "Recieve returned")

		},
	}

	rootCmd.PersistentFlags().BoolVarP(&debugMode, "debug", "d", false, "Enable detailed TCP and buffer logging")
	rootCmd.AddCommand(sendCmd, receiveCmd)
	rootCmd.Execute()
}
