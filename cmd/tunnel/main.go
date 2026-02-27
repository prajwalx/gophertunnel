package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/prajwalx/gophertunnel/internal/discovery"
	"github.com/prajwalx/gophertunnel/internal/security"
	"github.com/prajwalx/gophertunnel/internal/transfer"
	"github.com/spf13/cobra"
)

// fileExists checks if a file exists and is not a directory.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	// Check if it's a file, not a directory
	return err == nil && !info.IsDir()
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var rootCmd = &cobra.Command{Use: "tunnel"}

	var sendCmd = &cobra.Command{Use: "send [file]",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// file exist
			// open tunnel
			// wait
			path := args[0]
			const port = 8080
			const portStr = ":8080"

			if !fileExists(path) {
				fmt.Println("File do not exist in given path ", path)
				os.Exit(1)
			}

			//start mdns in background
			go discovery.StartServer(ctx, port)

			ln, err := net.Listen("tcp", portStr)
			if err != nil {
				fmt.Println("Unable to open tcp server on port ", portStr)
				os.Exit(1)
			}

			connChan := make(chan net.Conn, 1)

			go func() {
				conn, errCon := ln.Accept()
				if errCon != nil {
					fmt.Println("Unable to accept for receiver on ", portStr, " ...")
					os.Exit(1)
				}
				connChan <- conn
				// defer conn.Close()
			}()

			fmt.Printf("📦 Peer discovery active. Waiting for receiver...\n")
			fmt.Println("Waiting for receiver on ", portStr, " ...")

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

				transfer.SendHeader(conn, transfer.Metadata{FileName: fileInfo.Name(), Size: fileInfo.Size()})

				stream, err := security.GetStream(security.SharedKey)

				if err != nil {
					fmt.Println("Error getting cipher stream")
					os.Exit(1)
				}

				transfer.StreamFile(ctx, conn, path, stream)

			}

		},
	}

	var receiveCmd = &cobra.Command{Use: "receive",
		Run: func(cmd *cobra.Command, args []string) {
			ip, err := discovery.FindPeer(ctx)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			} else {
				fmt.Println("IP: ", ip)
			}

			const port = ":8080"
			conn, err := net.Dial("tcp", ip+port)
			if err != nil {
				fmt.Println("Unable to dial to tcp server on ", ip, port)
				os.Exit(1)
			}

			defer conn.Close()

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

			transfer.ReceiveFile(ctx, conn, meta.FileName, stream, meta.Size)
			fmt.Println("Recieve completed")

		},
	}

	rootCmd.AddCommand(sendCmd, receiveCmd)
	rootCmd.Execute()
}
