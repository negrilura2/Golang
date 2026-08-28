package logger

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"time"
)

var logger *zap.Logger
var atom = zap.NewAtomicLevelAt(zap.DebugLevel)

func init() {
	config := zap.Config{
		Level:       atom, // 改字段原子修改，可以动态更改日志等级
		Development: false,
		Encoding:    "json", // 指定 JSON 编码
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey: "msg",
			LevelKey:   "level",
			TimeKey:    "time",
			CallerKey:  "caller",
			//StacktraceKey: "stacktrace",
			EncodeTime:   zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
			EncodeLevel:  zapcore.LowercaseLevelEncoder,
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	tempLogger, err := config.Build()
	if err != nil {
		panic(err)
	}

	logger = tempLogger.WithOptions(zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zap.ErrorLevel))
}

type GormLogger struct {
	logger                    *zap.Logger
	LogLevel                  gormlogger.LogLevel
	SlowThreshold             time.Duration
	IgnoreRecordNotFoundError bool
}

func NewGormLogger() *GormLogger {
	return &GormLogger{
		logger:                    logger,
		LogLevel:                  gormlogger.Info,
		SlowThreshold:             100 * time.Millisecond,
		IgnoreRecordNotFoundError: false,
	}
}
func (l GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return GormLogger{
		SlowThreshold:             l.SlowThreshold,
		LogLevel:                  level,
		IgnoreRecordNotFoundError: l.IgnoreRecordNotFoundError,
	}
}

func (l GormLogger) writef(str string, args ...interface{}) string {
	return fmt.Sprintf(str, args...)
}

func (l GormLogger) Info(ctx context.Context, str string, args ...interface{}) {
	if l.LogLevel < gormlogger.Info {
		return
	}
	l.logger.Warn(l.writef(str, args...))
}

func (l GormLogger) Error(ctx context.Context, str string, args ...interface{}) {
	if l.LogLevel < gormlogger.Error {
		return
	}
	l.logger.Error(l.writef(str, args...))
}

func (l GormLogger) Warn(ctx context.Context, str string, args ...interface{}) {
	if l.LogLevel < gormlogger.Warn {
		return
	}
	l.logger.Warn(l.writef(str, args...))
}
func (l GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= 0 {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= gormlogger.Error && (!l.IgnoreRecordNotFoundError || !errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		logger.Error("gorm", zap.Error(err), zap.Duration("dur", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.LogLevel >= gormlogger.Warn:
		sql, rows := fc()
		logger.Warn("gorm", zap.Duration("dur", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	case l.LogLevel >= gormlogger.Info:
		sql, rows := fc()
		logger.Debug("gorm", zap.Duration("dur", elapsed), zap.Int64("rows", rows), zap.String("sql", sql))
	}
}

func GetLogger() *zap.Logger {
	return logger
}

func SetLevel(level string) {
	tLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		fmt.Printf("invalid level, input: %s", level)
		return
	}
	atom.SetLevel(tLevel)
}

func Debug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

func MaskToken(token string) string {
	if len(token) < 8 {
		return token
	}
	return token[:8] + "***"
}

type ctxKey struct{}

func WithRequestID(ctx context.Context, rid string) context.Context {
	return context.WithValue(ctx, ctxKey{}, rid)
}

func FromContext(ctx context.Context) *zap.Logger {
	if rid, ok := ctx.Value(ctxKey{}).(string); ok && rid != "" {
		return logger.With(zap.String("request_id", rid))
	}
	return logger
}
