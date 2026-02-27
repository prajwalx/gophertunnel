package logger

import (
	"fmt"
	"log"
)

func DebugLog(enabled bool, component string, format string, v ...interface{}) {
	if enabled {
		msg := fmt.Sprintf(format, v...)
		log.Printf("%-10s | %s", component, msg)
	}
}
