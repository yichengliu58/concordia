package util

import (
	"io"
	"log"
	"os"
)

const (
	DEBUG = iota
	INFO
	WARN
	ERROR
	PANIC
	FATAL
)

// a simple logger with features:
// - set prefix
// - support different levels
// - set different outputs
type Logger struct {
	l     *log.Logger
	level int
}

// default level is ERROR
func NewLogger(prefix string) *Logger {
	l := log.New(os.Stdout, prefix+" ", log.LstdFlags)

	return &Logger{
		l:     l,
		level: ERROR,
	}
}

func (l *Logger) SetOutput(output io.Writer) {
	l.l.SetOutput(output)
}

func (l *Logger) SetPrefix(prefix string) {
	l.l.SetPrefix(prefix + " ")
}

func (l *Logger) SetLevel(level int) {
	if level < DEBUG {
		l.level = DEBUG
	} else if level > PANIC {
		l.level = FATAL
	} else {
		l.level = level
	}
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.level <= DEBUG {
		l.l.Printf("[DEBUG] "+format+"\n", v...)
	}
}

func (l *Logger) Infof(format string, v ...interface{}) {
	if l.level <= INFO {
		l.l.Printf("[INFO] "+format+"\n", v...)
	}
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	if l.level <= WARN {
		l.l.Printf("[WARN] "+format+"\n", v...)
	}
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	if l.level <= ERROR {
		l.l.Printf("[ERROR] "+format+"\n", v...)
	}
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	if l.level <= FATAL {
		l.l.Fatalf("[FATAL] "+format, v...)
	}
}

func (l *Logger) Panicf(format string, v ...interface{}) {
	if l.level <= PANIC {
		l.l.Panicf("[PANIC] "+format, v...)
	}
}
