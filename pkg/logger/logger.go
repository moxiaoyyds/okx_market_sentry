package logger

import (
	"log"
	"os"
)

type Logger struct {
	*log.Logger
}

func New(level string) *Logger {
	logger := log.New(os.Stdout, "[OKX-SENTRY] ", log.LstdFlags|log.Lshortfile)
	return &Logger{Logger: logger}
}

func (l *Logger) Info(v ...interface{}) {
	l.SetPrefix("[INFO] ")
	l.Println(v...)
}

func (l *Logger) Warn(v ...interface{}) {
	l.SetPrefix("[WARN] ")
	l.Println(v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.SetPrefix("[ERROR] ")
	l.Println(v...)
}

func (l *Logger) Debug(v ...interface{}) {
	l.SetPrefix("[DEBUG] ")
	l.Println(v...)
}
