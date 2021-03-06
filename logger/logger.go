package logger

import (
	"io"
	"log"
	"os"

	"../config"
)

var (
	Trace *log.Logger
	Error *log.Logger
)

func Init() error {
	config, err := config.ReadConfig("")
	if err != nil {
		return err
	}
	logPath := config.LogPath + "log.txt"
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	traceW := io.Writer(os.Stdout)
	errorW := io.Writer(os.Stdout)
	if err != nil {
		log.Println("Failed to open log file")
	} else {
		traceW = io.Writer(file)
		errorW = io.MultiWriter(file, os.Stdout)
	}

	Trace = log.New(traceW,
		"[TRACE]: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(errorW,
		"[ERROR]: ",
		log.Ldate|log.Ltime|log.Lshortfile)
	return nil
}
