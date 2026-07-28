package zlog

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
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
	AddCallerSkip = zap.AddCallerSkip
)

// log文件后缀类型
const (
	txtLogNormal    = "normal"
	txtLogWarnFatal = "warnfatal"
	txtLogAccess    = "accesslog"
)

// 缓存基础 Zap core 和 Logger 实例
var (
	baseZapCore    zapcore.Core
	baseAccessCore zapcore.Core
	normalOnce     sync.Once
	accessOnce     sync.Once
)

// buildZapCore 构造 zapcore.Core，支持普通日志和 Access 日志类型
func buildZapCore(isAccess bool) zapcore.Core {
	encoder := getEncoder()
	name := logConfig.ModuleName
	if name == "" {
		name = "server"
	}
	// 普通日志 core
	if !isAccess {
		normalOnce.Do(func() {
			var infoLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= logConfig.ZapLevel && lvl <= zapcore.InfoLevel
			})
			var errorLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= logConfig.ZapLevel && lvl >= zapcore.WarnLevel
			})
			var stdLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= logConfig.ZapLevel && lvl >= zapcore.DebugLevel
			})

			var cores []zapcore.Core
			// 控制台输出
			cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), stdLevel))
			if logConfig.Log2File {
				cores = append(cores, zapcore.NewCore(encoder, getLogFileWriter(name, txtLogNormal), infoLevel))
				cores = append(cores, zapcore.NewCore(encoder, getLogFileWriter(name, txtLogWarnFatal), errorLevel))
			}
			baseZapCore = zapcore.NewTee(cores...)
		})
		return baseZapCore
	}

	// Access 日志 core
	accessOnce.Do(func() {
		var infoLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= logConfig.ZapLevel && lvl <= zapcore.InfoLevel
		})
		var stdLevel = zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= logConfig.ZapLevel && lvl >= zapcore.DebugLevel
		})

		var cores []zapcore.Core
		// 控制台输出
		cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), stdLevel))
		cores = append(cores, zapcore.NewCore(encoder, getLogFileWriter(name, txtLogAccess), infoLevel))
		baseAccessCore = zapcore.NewTee(cores...)
	})
	return baseAccessCore
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
	if logConfig.LogFormat == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}
	return &defaultEncoder{
		Encoder: encoder,
	}
}

type defaultEncoder struct {
	zapcore.Encoder
}

func (enc *defaultEncoder) Clone() zapcore.Encoder {
	encoderClone := enc.Encoder.Clone()
	return &defaultEncoder{Encoder: encoderClone}
}

func (enc *defaultEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	ent.Time = time.Now()
	return enc.Encoder.EncodeEntry(ent, fields)
}

func getLogFileWriter(name, loggerType string) (ws zapcore.WriteSyncer) {
	filename := filepath.Join(filepath.Clean(logConfig.Path), appendLogFileTail(name, loggerType))
	fileWriter := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    100,
		MaxBackups: 14,
		MaxAge:     14,
		Compress:   true,
		LocalTime:  true,
	}
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
func appendLogFileTail(appName, loggerType string) string {
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
	return appName + tailFixed
}
