package orm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	ormUtil "gorm.io/gorm/utils"

	"github.com/xiangtao94/golib/pkg/zlog"
)

var MemoryPromCollector prometheus.Collector

// MemoryConf 内存数据库配置
type MemoryConf struct {
	DatabaseName string `yaml:"database_name"` // 内存数据库名称，主要用于标识
}

func (conf *MemoryConf) checkConf() {
	if conf.DatabaseName == "" {
		conf.DatabaseName = "memory_db"
	}
}

// InitMemoryClient 初始化内存数据库客户端
func InitMemoryClient(conf MemoryConf) (client *gorm.DB, err error) {
	conf.checkConf()

	l := newMemoryLogger()
	c := &gorm.Config{
		SkipDefaultTransaction: true,
		FullSaveAssociations:   false,
		Logger:                 l,
	}

	// 使用SQLite内存数据库
	client, err = gorm.Open(sqlite.Open(":memory:"), c)
	if err != nil {
		return client, err
	}

	sqlDB, err := client.DB()
	if err != nil {
		return client, err
	}

	// 内存数据库不需要连接池设置，但为了保持接口一致性，这里保留
	// 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(1)
	// 设置打开数据库连接的最大数量
	sqlDB.SetMaxOpenConns(1)
	// 设置了连接可复用的最大时间
	sqlDB.SetConnMaxLifetime(time.Hour)
	// 设置最大空闲连接时间
	sqlDB.SetConnMaxIdleTime(time.Hour)

	MemoryPromCollector = collectors.NewDBStatsCollector(sqlDB, conf.DatabaseName)
	return client, nil
}

type memoryLogger struct {
	logger *zlog.Logger
}

func newMemoryLogger() *memoryLogger {
	return &memoryLogger{
		logger: zlog.NewLoggerWithSkip(3),
	}
}

func (l *memoryLogger) LogMode(level logger.LogLevel) logger.Interface {
	return l
}

// Info print info
func (l *memoryLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	m := fmt.Sprintf(msg, append([]interface{}{ormUtil.FileWithLineNum()}, data...)...)
	l.logger.Debug(m, l.AppendCustomField(ctx)...)
}

// Warn print warn messages
func (l *memoryLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	m := fmt.Sprintf(msg, append([]interface{}{ormUtil.FileWithLineNum()}, data...)...)
	l.logger.Warn(m, l.AppendCustomField(ctx)...)
}

// Error print error messages
func (l *memoryLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	m := fmt.Sprintf(msg, append([]interface{}{ormUtil.FileWithLineNum()}, data...)...)
	l.logger.Error(m, l.AppendCustomField(ctx)...)
}

// Trace print sql message
func (l *memoryLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	end := time.Now()
	// 请求是否成功
	msg := "memory_db"
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// 没有找到记录不统计在请求错误中
		msg = err.Error()
	}
	sql, rows := fc()
	fields := l.AppendCustomField(ctx)
	fields = append(fields,
		zlog.String("sql", sql),
		zlog.Int64("rows", rows),
	)
	fields = append(fields, zlog.AppendCostTime(begin, end)...)
	l.logger.Debug(msg, fields...)
}

func (l *memoryLogger) AppendCustomField(ctx context.Context) []zlog.Field {
	var requestID string
	if c, ok := ctx.(*gin.Context); ok && c != nil {
		requestID, _ = ctx.Value(zlog.ContextKeyRequestID).(string)
	}
	fields := []zlog.Field{
		zlog.String("requestId", requestID),
	}
	return fields
}

// MemoryTransactionManager 内存数据库事务管理器
type MemoryTransactionManager struct {
	ctx *gin.Context
	db  *gorm.DB
}

// NewMemoryTransactionManager 创建内存数据库事务管理器
func NewMemoryTransactionManager(ctx *gin.Context, client *gorm.DB) *MemoryTransactionManager {
	return &MemoryTransactionManager{
		ctx: ctx,
		db:  client.WithContext(ctx),
	}
}

// ExecuteInTransaction 在事务中执行操作
func (tm *MemoryTransactionManager) ExecuteInTransaction(operations ...func(*gorm.DB) error) error {
	tx := tm.db.Begin()

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	for _, operation := range operations {
		if err := operation(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("memory transaction execute error: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("memory transaction commit error: %w", err)
	}

	return nil
}
