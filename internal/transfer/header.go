package transfer

import (
	"encoding/json"
	"net"
)

type Metadata struct {
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
}

func SendHeader(conn net.Conn, meta Metadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = conn.Write(append(data, '\n'))
	return err
}
