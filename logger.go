package recache

import (
	"context"
	"fmt"
)

type logger interface {
	WithContext(ctx context.Context) logger
	Infof(fmt string, args ...any)
	Errorf(fmt string, args ...any)
	Warnf(fmt string, args ...any)
}

type defaultLogger struct{}

func (l *defaultLogger) WithContext(ctx context.Context) logger {
	return l
}

func (l *defaultLogger) Infof(format string, args ...any) {
	txt := fmt.Sprintf(format, args...)
	fmt.Printf("[Info] %s\n", txt)
}

func (l *defaultLogger) Errorf(format string, args ...any) {
	txt := fmt.Sprintf(format, args...)
	fmt.Printf("[Error] %s\n", txt)
}

func (l *defaultLogger) Warnf(format string, args ...any) {
	txt := fmt.Sprintf(format, args...)
	fmt.Printf("[Warn] %s\n", txt)
}
