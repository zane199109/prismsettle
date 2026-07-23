package logger

import (
	"os"
	"path/filepath"
	"time"

	"github.com/zane/web3-offchain/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log *zap.Logger = zap.NewNop() // Global logger instance; default no-op so library
// calls are safe before InitLogger() runs (e.g. in tests). InitLogger() replaces it.

// InitLogger initializes production-grade logger (daily rotation, level-based output)
func InitLogger() {
	// Log directory
	logPath := config.Cfg.Server.LogPath
	if err := os.MkdirAll(logPath, 0755); err != nil {
		panic("create log dir failed: " + err.Error())
	}

	// Log rotation configuration
	lumberjackLogger := &lumberjack.Logger{
		Filename:   filepath.Join(logPath, "web3-offchain.log"),
		MaxSize:    100,  // 100MB per file
		MaxBackups: 7,    // Keep 7 backups
		MaxAge:     30,   // Keep for 30 days
		Compress:   true, // Compress backups
	}

	// Encoder configuration
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Output levels
	level := zapcore.InfoLevel
	if config.Cfg.Env == "dev" {
		level = zapcore.DebugLevel
	}

	// Core configuration
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(lumberjackLogger)),
		level,
	)

	// Enable development mode (line numbers, stack traces)
	log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	zap.ReplaceGlobals(log)
}

// ==================== Correct structured logging methods (no garbled text!) ====================
func Info(msg string, fields ...zap.Field) {
	log.Info(msg, fields...)
}

func Errorf(msg string, fields ...zap.Field) {
	log.Error(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	log.Warn(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	log.Debug(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	log.Fatal(msg, fields...)
}

// ==================== Encapsulate commonly used zap.Field (consistent with official usage) ====================
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

func Uint64(key string, value uint64) zap.Field {
	return zap.Uint64(key, value)
}

func Uint32(key string, value uint32) zap.Field {
	return zap.Uint32(key, value)
}

func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

func Int64(key string, value int64) zap.Field {
	return zap.Int64(key, value)
}

func Duration(key string, value time.Duration) zap.Field {
	return zap.Duration(key, value)
}

func Any(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

func Error(err error) zap.Field {
	return zap.Error(err)
}

func Stack(key string) zap.Field {
	return zap.Stack(key)
}

func TraceID(traceID string) zap.Field {
	return zap.String("trace_id", traceID)
}

func Cost(start time.Time) zap.Field {
	return zap.Duration("cost", time.Since(start))
}
