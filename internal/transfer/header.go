package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
)

type Metadata struct {
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
}

func SendHeader(conn net.Conn, meta Metadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = conn.Write(append(data, '\n'))
	return err
}

// Pre-calculates the hash so it can be sent in the JSON header
func CalculateFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
