package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

// PersistenceMode 持久化模式
type PersistenceMode string

const (
	// PureMemo 纯内存模式，数据不持久化
	PureMemo PersistenceMode = "pure_memory"
	// FileMode 文件模式，数据直接存储在磁盘文件中
	FileMode PersistenceMode = "file"
	// MemoryWithBackup 内存+备份模式，数据在内存中，定期备份到磁盘
	MemoryWithBackup PersistenceMode = "memory_with_backup"
)

// MemoryConf 内存数据库配置
type MemoryConf struct {
	DatabaseName     string          `yaml:"database_name"`      // 数据库名称
	PersistenceMode  PersistenceMode `yaml:"persistence_mode"`   // 持久化模式
	FilePath         string          `yaml:"file_path"`          // 文件路径（FileMode和MemoryWithBackup模式使用）
	BackupInterval   time.Duration   `yaml:"backup_interval"`    // 备份间隔（MemoryWithBackup模式使用）
	AutoBackup       bool            `yaml:"auto_backup"`        // 是否自动备份
	BackupOnShutdown bool            `yaml:"backup_on_shutdown"` // 关闭时是否备份
}

func (conf *MemoryConf) checkConf() {
	if conf.DatabaseName == "" {
		conf.DatabaseName = "memory_db"
	}
	if conf.PersistenceMode == "" {
		conf.PersistenceMode = PureMemo
	}
	if conf.FilePath == "" {
		conf.FilePath = fmt.Sprintf("%s.db", conf.DatabaseName)
	}
	if conf.BackupInterval == 0 {
		conf.BackupInterval = 5 * time.Minute // 默认5分钟备份一次
	}
}

// MemoryDB 内存数据库结构体
type MemoryDB struct {
	*gorm.DB
	conf       MemoryConf
	backupStop chan bool
	backupWg   sync.WaitGroup
}

// InitMemoryDB 初始化内存数据库客户端
func InitMemoryDB(conf MemoryConf) (*MemoryDB, error) {
	conf.checkConf()

	l := newMemoryLogger()
	c := &gorm.Config{
		SkipDefaultTransaction: true,
		FullSaveAssociations:   false,
		Logger:                 l,
	}

	var client *gorm.DB
	var err error

	switch conf.PersistenceMode {
	case PureMemo:
		// 纯内存模式
		client, err = gorm.Open(sqlite.Open(":memory:"), c)
	case FileMode:
		// 文件模式
		err = os.MkdirAll(filepath.Dir(conf.FilePath), 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
		client, err = gorm.Open(sqlite.Open(conf.FilePath), c)
	case MemoryWithBackup:
		// 内存+备份模式
		client, err = gorm.Open(sqlite.Open(":memory:"), c)
		if err == nil {
			// 如果备份文件存在，则恢复数据
			if _, fileErr := os.Stat(conf.FilePath); fileErr == nil {
				err = restoreFromBackup(client, conf.FilePath)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported persistence mode: %s", conf.PersistenceMode)
	}

	if err != nil {
		return nil, err
	}

	sqlDB, err := client.DB()
	if err != nil {
		return nil, err
	}

	// 设置连接池参数
	if conf.PersistenceMode == FileMode {
		// 文件模式可以设置更多连接
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetMaxOpenConns(10)
	} else {
		// 内存模式保持较少连接
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetMaxOpenConns(3)
	}
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(time.Hour)

	MemoryPromCollector = collectors.NewDBStatsCollector(sqlDB, conf.DatabaseName)

	memDB := &MemoryDB{
		DB:   client,
		conf: conf,
	}

	// 启动自动备份（仅对MemoryWithBackup模式）
	if conf.PersistenceMode == MemoryWithBackup && conf.AutoBackup {
		memDB.startAutoBackup()
	}

	return memDB, nil
}

// GetDB 获取GORM数据库实例（兼容性方法）
func (m *MemoryDB) GetDB() *gorm.DB {
	return m.DB
}

// BackupToFile 手动备份数据到文件
func (m *MemoryDB) BackupToFile(filePath string) error {
	if m.conf.PersistenceMode == FileMode {
		return fmt.Errorf("file mode doesn't need backup")
	}

	return backupToFile(m.DB, filePath)
}

// RestoreFromFile 从文件恢复数据
func (m *MemoryDB) RestoreFromFile(filePath string) error {
	if m.conf.PersistenceMode == FileMode {
		return fmt.Errorf("file mode doesn't need restore")
	}

	return restoreFromBackup(m.DB, filePath)
}

// StartAutoBackup 启动自动备份
func (m *MemoryDB) StartAutoBackup() {
	if m.conf.PersistenceMode != MemoryWithBackup {
		return
	}
	m.startAutoBackup()
}

// StopAutoBackup 停止自动备份
func (m *MemoryDB) StopAutoBackup() {
	if m.backupStop != nil {
		close(m.backupStop)
		m.backupWg.Wait()
		m.backupStop = nil
	}
}

// Close 关闭数据库连接
func (m *MemoryDB) Close() error {
	// 如果配置了关闭时备份，则执行备份
	if m.conf.PersistenceMode == MemoryWithBackup && m.conf.BackupOnShutdown {
		if err := m.BackupToFile(m.conf.FilePath); err != nil {
			// 创建一个简单的gin.Context来满足zlog的要求
			ctx := &gin.Context{}
			zlog.Errorf(ctx, "Failed to backup on shutdown: %v", err)
		}
	}

	// 停止自动备份
	m.StopAutoBackup()

	// 关闭数据库连接
	sqlDB, err := m.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// 启动自动备份goroutine
func (m *MemoryDB) startAutoBackup() {
	if m.backupStop != nil {
		return // 已经启动了
	}

	m.backupStop = make(chan bool)
	m.backupWg.Add(1)

	go func() {
		defer m.backupWg.Done()
		ticker := time.NewTicker(m.conf.BackupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := m.BackupToFile(m.conf.FilePath); err != nil {
					// 创建一个简单的gin.Context来满足zlog的要求
					ctx := &gin.Context{}
					zlog.Errorf(ctx, "Auto backup failed: %v", err)
				} else {
					// 创建一个简单的gin.Context来满足zlog的要求
					ctx := &gin.Context{}
					zlog.Debugf(ctx, "Auto backup completed to: %s", m.conf.FilePath)
				}
			case <-m.backupStop:
				return
			}
		}
	}()
}

// backupToFile 备份内存数据库到文件
func backupToFile(memDB *gorm.DB, filePath string) error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 获取内存数据库的底层连接
	sqlDB, err := memDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// 创建文件数据库
	fileDB, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return fmt.Errorf("failed to open file database: %w", err)
	}
	defer fileDB.Close()

	// 使用GORM的Migrator获取所有表名
	migrator := memDB.Migrator()
	tables, err := migrator.GetTables()
	if err != nil {
		return fmt.Errorf("failed to get tables: %w", err)
	}

	// 开始事务
	tx, err := fileDB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 复制每个表的结构和数据
	for _, table := range tables {
		// 跳过SQLite系统表
		if table == "sqlite_sequence" || table == "sqlite_stat1" || table == "sqlite_stat4" {
			continue
		}

		// 获取表结构
		var createSQL string
		err = sqlDB.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&createSQL)
		if err != nil {
			continue // 跳过系统表等
		}

		// 删除已存在的表
		tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table))

		// 创建表结构
		if _, err = tx.Exec(createSQL); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table, err)
		}

		// 复制数据
		rows, err := sqlDB.Query(fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			continue
		}

		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			continue
		}

		// 准备插入语句
		placeholders := make([]string, len(columns))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		insertSQL := fmt.Sprintf("INSERT INTO %s VALUES (%s)", table,
			fmt.Sprintf("%s", placeholders[0]))
		for i := 1; i < len(placeholders); i++ {
			insertSQL = insertSQL[:len(insertSQL)-1] + "," + placeholders[i] + ")"
		}

		stmt, err := tx.Prepare(insertSQL)
		if err != nil {
			rows.Close()
			continue
		}

		// 复制每一行数据
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		for rows.Next() {
			if err := rows.Scan(valuePtrs...); err != nil {
				continue
			}
			if _, err := stmt.Exec(values...); err != nil {
				continue
			}
		}
		stmt.Close()
		rows.Close()
	}

	return tx.Commit()
}

// restoreFromBackup 从备份文件恢复数据到内存数据库
func restoreFromBackup(memDB *gorm.DB, filePath string) error {
	// 检查备份文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", filePath)
	}

	// 打开备份文件
	fileDB, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer fileDB.Close()

	// 获取内存数据库连接
	sqlDB, err := memDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get memory database: %w", err)
	}

	// 获取所有表名
	rows, err := fileDB.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("failed to get tables from backup: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, tableName)
	}

	// 开始事务
	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 恢复每个表
	for _, table := range tables {
		// 跳过SQLite系统表
		if table == "sqlite_sequence" || table == "sqlite_stat1" || table == "sqlite_stat4" {
			continue
		}

		// 获取表结构
		var createSQL string
		err = fileDB.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&createSQL)
		if err != nil {
			continue
		}

		// 删除已存在的表，重新创建
		tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table))

		// 创建表结构
		if _, err = tx.Exec(createSQL); err != nil {
			continue
		}

		// 复制数据
		dataRows, err := fileDB.Query(fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			continue
		}

		columns, err := dataRows.Columns()
		if err != nil {
			dataRows.Close()
			continue
		}

		// 准备插入语句
		placeholders := make([]string, len(columns))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		insertSQL := fmt.Sprintf("INSERT OR REPLACE INTO %s VALUES (%s)", table,
			fmt.Sprintf("%s", placeholders[0]))
		for i := 1; i < len(placeholders); i++ {
			insertSQL = insertSQL[:len(insertSQL)-1] + "," + placeholders[i] + ")"
		}

		stmt, err := tx.Prepare(insertSQL)
		if err != nil {
			dataRows.Close()
			continue
		}

		// 复制每一行数据
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		for dataRows.Next() {
			if err := dataRows.Scan(valuePtrs...); err != nil {
				continue
			}
			if _, err := stmt.Exec(values...); err != nil {
				continue
			}
		}
		stmt.Close()
		dataRows.Close()
	}

	return tx.Commit()
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
		zlog.String("cost", fmt.Sprintf("%v%s", zlog.GetRequestCost(begin, end), "ms")),
	)
	l.logger.Debug(msg, fields...)
}

func (l *memoryLogger) AppendCustomField(ctx context.Context) []zlog.Field {
	var requestID string
	if c, ok := ctx.(*gin.Context); ok && c != nil {
		requestID = zlog.GetRequestID(c)
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

// 兼容性函数，保持向后兼容
func InitMemoryClient(conf MemoryConf) (*gorm.DB, error) {
	memDB, err := InitMemoryDB(conf)
	if err != nil {
		return nil, err
	}
	return memDB.GetDB(), nil
}
