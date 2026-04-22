package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

type Logger struct {
	base *log.Logger
	file *log.Logger
}

func New(file io.Writer) *Logger {
	return &Logger{
		base: log.New(os.Stdout, "", 0),
		file: log.New(file, "", 0),
	}
}

func (l *Logger) Info(message string) {
	l.log("info", message)
}

func (l *Logger) Warn(message string) {
	l.log("warn", message)
}

func (l *Logger) Error(message string) {
	l.log("error", message)
}

func (l *Logger) log(level, message string) {
	line := fmt.Sprintf("%s [%s]: %s", time.Now().Format("2006-01-02 15:04:05"), level, message)
	l.base.Println(line)
	l.file.Println(line)
}
