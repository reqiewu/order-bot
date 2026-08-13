package applog

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger — slog-like API поверх zap (цветные уровни в консоли).
type Logger struct {
	s     *zap.SugaredLogger
	level zapcore.Level
}

// KV — пара для InfoTree / DebugTree.
type KV struct {
	K string
	V string
}

// ParseLevel — debug|info|warn|error (default info).
func ParseLevel(s string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zap.DebugLevel
	case "warn", "warning":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

// New создаёт console-логгер. debug=true → DebugLevel (совместимость).
func New(debug bool) *Logger {
	if debug {
		return NewLevel("debug")
	}
	return NewLevel("info")
}

// NewLevel — уровень из строки (debug|info|warn|error).
func NewLevel(level string) *Logger {
	lvl := ParseLevel(level)
	encCfg := zap.NewDevelopmentEncoderConfig()
	encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encCfg.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
	encCfg.ConsoleSeparator = "  "
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encCfg),
		zapcore.AddSync(os.Stdout),
		lvl,
	)
	z := zap.New(core, zap.AddCallerSkip(1))
	return &Logger{s: z.Sugar(), level: lvl}
}

// Nop — silent logger for tests.
func Nop() *Logger {
	return &Logger{s: zap.NewNop().Sugar(), level: zapcore.InvalidLevel}
}

func (l *Logger) sugar() *zap.SugaredLogger {
	if l == nil || l.s == nil {
		return zap.NewNop().Sugar()
	}
	return l.s
}

// DebugEnabled — true если LOG_LEVEL=debug.
func (l *Logger) DebugEnabled() bool {
	return l != nil && l.level == zap.DebugLevel
}

func (l *Logger) Debug(msg string, kv ...any) { l.sugar().Debugw(msg, kv...) }
func (l *Logger) Info(msg string, kv ...any)  { l.sugar().Infow(msg, kv...) }
func (l *Logger) Warn(msg string, kv ...any)  { l.sugar().Warnw(msg, kv...) }
func (l *Logger) Error(msg string, kv ...any) { l.sugar().Errorw(msg, kv...) }

// InfoTree — заголовок INFO + ветки ├── / └── (без JSON).
func (l *Logger) InfoTree(msg string, kvs ...KV) {
	l.tree(zap.InfoLevel, msg, kvs...)
}

// DebugTree — то же на DEBUG (молчает, если уровень выше).
func (l *Logger) DebugTree(msg string, kvs ...KV) {
	l.tree(zap.DebugLevel, msg, kvs...)
}

func (l *Logger) tree(level zapcore.Level, msg string, kvs ...KV) {
	if l == nil || l.s == nil {
		return
	}
	if l.level > level {
		return
	}
	switch level {
	case zap.DebugLevel:
		l.Debug(msg)
	default:
		l.Info(msg)
	}
	n := len(kvs)
	for i, kv := range kvs {
		branch := "├──"
		if i == n-1 {
			branch = "└──"
		}
		fmt.Fprintf(os.Stdout, "  %s %s: %s\n", branch, kv.K, kv.V)
	}
}

func (l *Logger) Sync() {
	_ = l.sugar().Sync()
}
