package zlog

import (
	"github.com/gin-gonic/gin"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/xiangtao94/golib/pkg/env"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	Field  = zap.Field
	Logger = zap.Logger
)

var (
	Binary = zap.Binary
	Bool   = zap.Bool

	ByteString = zap.ByteString
	String     = zap.String
	Strings    = zap.Strings

	Float64 = zap.Float64
	Float32 = zap.Float32

	Int   = zap.Int
	Int64 = zap.Int64
	Int32 = zap.Int32
	Int16 = zap.Int16
	Int8  = zap.Int8

	Uint   = zap.Uint
	Uint64 = zap.Uint64
	Uint32 = zap.Uint32

	Reflect       = zap.Reflect
	Namespace     = zap.Namespace
	Duration      = zap.Duration
	Object        = zap.Object
	Any           = zap.Any
	Skip          = zap.Skip()
	AddCallerSkip = zap.AddCallerSkip
)
var (
	SugaredLogger *zap.SugaredLogger
	ZapLogger     *zap.Logger
)

// log文件后缀类型
const (
	txtLogNormal    = "normal"
	txtLogWarnFatal = "warnfatal"
	txtLogAccess    = "accesslog"
	txtLogStdout    = "stdout"
)

// NewLogger 新建Logger，每一次新建会同时创建x.log与x.log.wf (access.log 不会生成wf)
func newLogger() *zap.Logger {
	var infoLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= logConfig.ZapLevel && lvl <= zapcore.InfoLevel
	})

	var errorLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= logConfig.ZapLevel && lvl >= zapcore.WarnLevel
	})

	var stdLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= logConfig.ZapLevel && lvl >= zapcore.DebugLevel
	})

	encoder := getEncoder()
	name := logConfig.ModuleName
	if name == "" {
		name = "server"
	}
	var zapCore []zapcore.Core
	// 控制台输出
	stdoutCore := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), stdLevel)
	zapCore = append(zapCore, stdoutCore)
	if logConfig.Log2File {
		zapCore = append(zapCore, zapcore.NewCore(encoder, getLogFileWriter(name, txtLogNormal), infoLevel))
		zapCore = append(zapCore, zapcore.NewCore(encoder, getLogFileWriter(name, txtLogWarnFatal), errorLevel))
	}
	// core
	core := zapcore.NewTee(zapCore...)
	// 开启开发模式，堆栈跟踪
	caller := zap.WithCaller(true)
	development := zap.Development()
	filed := zap.Fields()
	logger := zap.New(core, filed, caller, development)

	return logger
}

// NewLogger 新建Logger，每一次新建会同时创建x.log与x.log.wf (access.log 不会生成wf)
func newAccessLogger() *zap.Logger {
	var infoLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= logConfig.ZapLevel && lvl <= zapcore.InfoLevel
	})
	encoder := getEncoder()
	name := logConfig.ModuleName
	if name == "" {
		name = "server"
	}
	var zapCore []zapcore.Core
	zapCore = append(zapCore, zapcore.NewCore(encoder, getLogFileWriter(name, txtLogAccess), infoLevel))
	// core
	core := zapcore.NewTee(zapCore...)
	// 开启开发模式，堆栈跟踪
	caller := zap.WithCaller(true)
	development := zap.Development()
	filed := zap.Fields()
	logger := zap.New(core, filed, caller, development)
	return logger
}

func GetAccessLogger() (l *zap.Logger) {
	if ZapLogger == nil {
		ZapLogger = newAccessLogger().WithOptions(zap.AddCallerSkip(1))
	}
	return ZapLogger
}

func zapAccessLogger(ctx *gin.Context) *zap.Logger {
	m := GetAccessLogger()
	if ctx == nil {
		return m
	}
	if t, exist := ctx.Get(zapAccessLoggerAddr); exist {
		if l, ok := t.(*zap.Logger); ok {
			return l
		}
	}

	l := m.With(
		zap.String("requestId", GetRequestID(ctx)),
		zap.String("localIp", env.LocalIP),
		zap.String("uri", GetRequestUri(ctx)),
	)

	ctx.Set(zapAccessLoggerAddr, l)
	return l
}

func AccessLogger(ctx *gin.Context, msg string, fields ...zap.Field) {
	zapAccessLogger(ctx).Info(msg, fields...)
}

func getLogLevel(lv string) (level zapcore.Level) {
	str := strings.ToUpper(lv)
	switch str {
	case "DEBUG":
		level = zap.DebugLevel
	case "INFO":
		level = zap.InfoLevel
	case "WARN":
		level = zap.WarnLevel
	case "ERROR":
		level = zap.ErrorLevel
	case "FATAL":
		level = zap.FatalLevel
	default:
		level = zap.InfoLevel
	}
	return level
}

func getEncoder() zapcore.Encoder {
	// time字段编码器
	timeEncoder := zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.999")
	encoderCfg := zapcore.EncoderConfig{
		LevelKey:       "level",
		TimeKey:        "time",
		CallerKey:      "file",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeCaller:   zapcore.ShortCallerEncoder, // 短路径编码器
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     timeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	var encoder zapcore.Encoder
	if logConfig.LogFormat == "json" {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}
	return &defaultEncoder{
		Encoder: encoder,
	}
}

func getLogFileWriter(name, loggerType string) (ws zapcore.WriteSyncer) {
	logDir := strings.TrimSuffix(logConfig.Path, "/")
	filenamePattern := filepath.Join(logDir, appendLogFileTail(name, loggerType, true))
	filename := filepath.Join(logDir, appendLogFileTail(name, loggerType, false))
	// Info按日期切割日志，每天一个新文件
	fileWriter, _ := rotatelogs.New(
		filenamePattern,                           // 生成的日志文件格式
		rotatelogs.WithLinkName(filename),         // 软链接，指向最新日志
		rotatelogs.WithMaxAge(14*24*time.Hour),    // 只保留 14 天的日志
		rotatelogs.WithRotationTime(24*time.Hour), // 每 24 小时切割一次
	)
	if !logConfig.BufferSwitch {
		return zapcore.AddSync(fileWriter)
	}
	// 开启缓冲区
	ws = &zapcore.BufferedWriteSyncer{
		WS:            zapcore.AddSync(fileWriter),
		Size:          logConfig.BufferSize,
		FlushInterval: logConfig.BufferFlushInterval,
		Clock:         nil,
	}
	return ws
}

// genFilename 拼装完整文件名
func appendLogFileTail(appName, loggerType string, pattern bool) string {
	var tailFixed string
	switch loggerType {
	case txtLogNormal:
		tailFixed = ".log"
	case txtLogWarnFatal:
		tailFixed = ".log.wf"
	case txtLogAccess:
		tailFixed = ".log.access"
	default:
		tailFixed = ".log"
	}
	if pattern {
		return appName + "-%Y-%m-%d" + tailFixed
	}
	return appName + tailFixed
}

func CloseLogger() {
	if SugaredLogger != nil {
		_ = SugaredLogger.Sync()
	}

	if ZapLogger != nil {
		_ = ZapLogger.Sync()
	}
}

/*---------------zapLogger-------------------*/

func GetZapLogger() (l *zap.Logger) {
	if ZapLogger == nil {
		ZapLogger = newLogger().WithOptions(zap.AddCallerSkip(1))
	}
	return ZapLogger
}

func zapLogger(ctx *gin.Context) *zap.Logger {
	m := GetZapLogger()
	if ctx == nil {
		return m
	}
	if t, exist := ctx.Get(zapLoggerAddr); exist {
		if l, ok := t.(*zap.Logger); ok {
			return l
		}
	}

	l := m.With(
		zap.String("requestId", GetRequestID(ctx)),
		zap.String("localIp", env.LocalIP),
		zap.String("uri", GetRequestUri(ctx)),
	)

	ctx.Set(zapLoggerAddr, l)
	return l
}

func DebugLogger(ctx *gin.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Debug(msg, fields...)
}
func InfoLogger(ctx *gin.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Info(msg, fields...)
}

func WarnLogger(ctx *gin.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Warn(msg, fields...)
}

func ErrorLogger(ctx *gin.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Error(msg, fields...)
}

func PanicLogger(ctx *gin.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Panic(msg, fields...)
}

func FatalLogger(ctx *gin.Context, msg string, fields ...zap.Field) {
	if noLog(ctx) {
		return
	}
	zapLogger(ctx).Fatal(msg, fields...)
}

/*---------------sugar Logger-------------------*/

func GetLogger() (s *zap.SugaredLogger) {
	if SugaredLogger != nil {
		return SugaredLogger
	}
	if ZapLogger != nil {
		SugaredLogger = ZapLogger.Sugar()
		return SugaredLogger
	}
	ZapLogger = GetZapLogger()
	SugaredLogger = ZapLogger.Sugar()
	return SugaredLogger
}

// 通用字段封装
func sugaredLogger(ctx *gin.Context) *zap.SugaredLogger {
	if ctx == nil {
		return SugaredLogger
	}

	if t, exist := ctx.Get(sugaredLoggerAddr); exist {
		if s, ok := t.(*zap.SugaredLogger); ok {
			return s
		}
	}

	s := SugaredLogger.With(
		zap.String("localIp", env.LocalIP),
		zap.String("uri", GetRequestUri(ctx)),
		zap.String("requestId", GetRequestID(ctx)),
	)
	ctx.Set(sugaredLoggerAddr, s)
	return s
}

func Debugf(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Debugf(format, args...)
}

func Info(ctx *gin.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Info(args...)
}

func Infof(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Infof(format, args...)
}

func Warn(ctx *gin.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Warn(args...)
}

func Warnf(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Warnf(format, args...)
}

func Error(ctx *gin.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Error(args...)
}

func Errorf(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Errorf(format, args...)
}

func Panic(ctx *gin.Context, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Panic(args...)
}

func Panicf(ctx *gin.Context, format string, args ...interface{}) {
	if noLog(ctx) {
		return
	}
	sugaredLogger(ctx).Panicf(format, args...)
}
