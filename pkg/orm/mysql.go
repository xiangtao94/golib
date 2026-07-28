package orm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	ormUtil "gorm.io/gorm/utils"

	"github.com/xiangtao94/golib/pkg/zlog"
)

type MysqlConf struct {
	DataBase        string        `yaml:"database"`
	Addr            string        `yaml:"addr"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Charset         string        `yaml:"charset"`
	MaxIdleConns    int           `yaml:"maxidleconns"`
	MaxOpenConns    int           `yaml:"maxopenconns"`
	ConnMaxIdlTime  time.Duration `yaml:"maxIdleTime"`
	ConnMaxLifeTime time.Duration `yaml:"connMaxLifeTime"`
	ConnTimeOut     time.Duration `yaml:"connTimeOut"`
	WriteTimeOut    time.Duration `yaml:"writeTimeOut"`
	ReadTimeOut     time.Duration `yaml:"readTimeOut"`
	TLSConfigName   string        `yaml:"tlsConfigName"`
	// SkipDefaultTransaction is an explicit performance opt-in. Keep false for
	// payment and other correctness-sensitive write paths.
	SkipDefaultTransaction bool `yaml:"skipDefaultTransaction"`
	// AllowInsecureTransport must be explicitly enabled for plaintext,
	// preferred, or certificate-skipping connections.
	AllowInsecureTransport bool `yaml:"allowInsecureTransport"`
}

func (conf *MysqlConf) checkConf() {
	if conf.MaxIdleConns == 0 {
		conf.MaxIdleConns = 50
	}
	if conf.MaxOpenConns == 0 {
		conf.MaxOpenConns = 50
	}
	if conf.ConnMaxIdlTime == 0 {
		conf.ConnMaxIdlTime = 5 * time.Minute
	}
	if conf.ConnMaxLifeTime == 0 {
		conf.ConnMaxLifeTime = 10 * time.Minute
	}
	if conf.ConnTimeOut == 0 {
		conf.ConnTimeOut = 3 * time.Second
	}
	if conf.WriteTimeOut == 0 {
		conf.WriteTimeOut = 1200 * time.Millisecond
	}
	if conf.ReadTimeOut == 0 {
		conf.ReadTimeOut = 1200 * time.Millisecond
	}

}

func InitMysqlClient(conf MysqlConf) (client *gorm.DB, err error) {
	conf.checkConf()
	dsn, err := buildMySQLDSN(conf)
	if err != nil {
		return nil, err
	}
	l := newLogger()
	c := gormConfig(conf, l)
	_ = driver.SetLogger(l)

	client, err = gorm.Open(mysql.Open(dsn), c)
	if err != nil {
		return client, err
	}

	sqlDB, err := client.DB()
	if err != nil {
		return client, err
	}
	// 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(conf.MaxIdleConns)
	// 设置打开数据库连接的最大数量
	sqlDB.SetMaxOpenConns(conf.MaxOpenConns)
	// 设置了连接可复用的最大时间
	sqlDB.SetConnMaxLifetime(conf.ConnMaxLifeTime)
	// 设置最大空闲连接时间
	sqlDB.SetConnMaxIdleTime(conf.ConnMaxIdlTime)
	return client, nil
}

func newGORMConfig(conf MysqlConf) *gorm.Config {
	return gormConfig(conf, newLogger())
}

func gormConfig(conf MysqlConf, log logger.Interface) *gorm.Config {
	return &gorm.Config{
		SkipDefaultTransaction: conf.SkipDefaultTransaction,
		FullSaveAssociations:   false,
		Logger:                 log,
	}
}

func buildMySQLDSN(conf MysqlConf) (string, error) {
	tlsMode := strings.TrimSpace(conf.TLSConfigName)
	insecureMode := tlsMode == "" ||
		strings.EqualFold(tlsMode, "false") ||
		strings.EqualFold(tlsMode, "preferred") ||
		strings.EqualFold(tlsMode, "skip-verify")
	if insecureMode && !conf.AllowInsecureTransport {
		return "", errors.New("mysql: TLS verification is required; configure tlsConfigName or explicitly allow insecure transport")
	}

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return "", fmt.Errorf("load mysql location: %w", err)
	}
	config := driver.Config{
		User:         conf.User,
		Passwd:       conf.Password,
		Net:          "tcp",
		Addr:         conf.Addr,
		DBName:       conf.DataBase,
		Timeout:      conf.ConnTimeOut,
		ReadTimeout:  conf.ReadTimeOut,
		WriteTimeout: conf.WriteTimeOut,
		ParseTime:    true,
		Loc:          location,
		TLSConfig:    tlsMode,
	}
	if conf.Charset != "" {
		config.Params = map[string]string{"charset": conf.Charset}
	}
	return config.FormatDSN(), nil
}

func NewMySQLPrometheusCollector(client *gorm.DB, name string) (prometheus.Collector, error) {
	if client == nil {
		return nil, errors.New("mysql client is nil")
	}
	sqlDB, err := client.DB()
	if err != nil {
		return nil, err
	}
	return collectors.NewDBStatsCollector(sqlDB, name), nil
}

type ormLogger struct {
	logger *zlog.Logger
}

func newLogger() *ormLogger {
	return &ormLogger{
		logger: zlog.NewLoggerWithSkip(3),
	}
}

// go-sql-driver error log
func (l *ormLogger) Print(args ...interface{}) {
	l.logger.Error(fmt.Sprint(args...), l.AppendCustomField(context.Background())...)
}

func (l *ormLogger) LogMode(level logger.LogLevel) logger.Interface {
	return l
}

// Info print info
func (l *ormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	m := fmt.Sprintf(msg, append([]interface{}{ormUtil.FileWithLineNum()}, data...)...)
	l.logger.Debug(m, l.AppendCustomField(ctx)...)
}

// Warn print warn messages
func (l *ormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	m := fmt.Sprintf(msg, append([]interface{}{ormUtil.FileWithLineNum()}, data...)...)
	l.logger.Warn(m, l.AppendCustomField(ctx)...)
}

// Error print error messages
func (l *ormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	m := fmt.Sprintf(msg, append([]interface{}{ormUtil.FileWithLineNum()}, data...)...)
	l.logger.Error(m, l.AppendCustomField(ctx)...)
}

// Trace print sql message
func (l *ormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	end := time.Now()
	// 请求是否成功
	msg := "mysql"
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

func (l *ormLogger) AppendCustomField(ctx context.Context) []zlog.Field {
	fields := []zlog.Field{
		zlog.String("requestId", zlog.GetRequestID(ctx)),
	}
	return fields
}
