package logging

import (
	"fmt"
	"time"
)

type LogLevel int

const (
	Error LogLevel = iota
	Debug
)

type Logger struct {
	level LogLevel
}

func (l *Logger) SetLogLevel(level LogLevel) {
	l.level = level
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.level >= Debug {
		now := time.Now().Format(time.UnixDate)
		fmt.Printf(now + ": " + format, args...)
	}
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	now := time.Now().Format(time.UnixDate)
	fmt.Printf(now + ": " + format, args...)
}

func NewLogger() Logger {
	return Logger{Error}
}

