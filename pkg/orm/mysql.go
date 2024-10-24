package orm

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/tiant-go/golib/pkg/zlog"
	ormUtil "gorm.io/gorm/utils"
	"gorm.io/plugin/dbresolver"
	"time"

	"github.com/gin-gonic/gin"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type CrudModel struct {
	CreatedAt time.Time      `json:"createdAt" gorm:"comment:创建时间"`
	UpdatedAt time.Time      `json:"updatedAt" gorm:"comment:最后更新时间"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index;comment:删除时间"`
}

type NormalPage struct {
	No      int    // 当前第几页
	Size    int    // 每页大小
	OrderBy string `json:"orderBy"` // 排序规则
}

type Option struct {
	IsNeedCnt  bool `json:"isNeedCnt"`
	IsNeedPage bool `json:"isNeedPage"`
}

// 分页示例
func NormalPaginate(page *NormalPage) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		pageNo := 1
		if page.No > 0 {
			pageNo = page.No
		}

		pageSize := page.Size
		switch {
		case pageSize > 100:
			pageSize = 100
		case pageSize <= 0:
			pageSize = 10
		}

		offset := (pageNo - 1) * pageSize
		orderBy := "id asc"
		if len(page.OrderBy) > 0 {
			orderBy = page.OrderBy
		}
		return db.Order(orderBy).Offset(offset).Limit(pageSize)
	}
}

type MysqlConf struct {
	DataBase        string        `yaml:"database"`
	Addr            string        `yaml:"addr"`
	SlaveAddrs      []string      `yaml:"slaveAddrs"`
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

	// sql 字段最大长度
	MaxSqlLen int `yaml:"maxSqlLen"`
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
	if conf.MaxSqlLen == 0 {
		// 日志中sql字段长度：
		// 如果不指定使用默认2048；如果<0表示不展示sql语句；否则使用用户指定的长度，过长会被截断
		conf.MaxSqlLen = 2048
	}
}

func InitMysqlClient(conf MysqlConf) (client *gorm.DB, err error) {
	conf.checkConf()
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?timeout=%s&readTimeout=%s&writeTimeout=%s&parseTime=True&loc=Asia%%2FShanghai",
		conf.User,
		conf.Password,
		conf.Addr,
		conf.DataBase,
		conf.ConnTimeOut,
		conf.ReadTimeOut,
		conf.WriteTimeOut)
	dsnArr := []string{}
	for _, s := range conf.SlaveAddrs {
		dsn2 := fmt.Sprintf("%s:%s@tcp(%s)/%s?timeout=%s&readTimeout=%s&writeTimeout=%s&parseTime=True&loc=Asia%%2FShanghai",
			conf.User,
			conf.Password,
			s,
			conf.DataBase,
			conf.ConnTimeOut,
			conf.ReadTimeOut,
			conf.WriteTimeOut)
		dsnArr = append(dsnArr, dsn2)
	}
	dsnArr = append(dsnArr, dsn)
	if conf.Charset != "" {
		dsn = dsn + "&charset=" + conf.Charset
	}

	l := newLogger()
	_ = driver.SetLogger(l)
	c := &gorm.Config{
		SkipDefaultTransaction: true,
		FullSaveAssociations:   false,
		Logger:                 l,
	}

	client, err = gorm.Open(mysql.Open(dsn), c)
	if err != nil {
		return client, err
	}
	var p []gorm.Dialector
	for _, s := range dsnArr {
		p = append(p, mysql.Open(s))
	}
	err = client.Use(dbresolver.Register(dbresolver.Config{
		// `db2` 作为 sources，`db3`、`db4` 作为 replicas
		Sources:  []gorm.Dialector{mysql.Open(dsn)},
		Replicas: p,
		// sources/replicas 负载均衡策略
		Policy: dbresolver.RandomPolicy{},
	}))
	if err != nil {
		return nil, err
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

type ormLogger struct {
	logger *zlog.Logger
}

func newLogger() *ormLogger {
	return &ormLogger{
		logger: zlog.ZapLogger.WithOptions(zlog.AddCallerSkip(2)),
	}
}

// go-sql-driver error log
func (l *ormLogger) Print(args ...interface{}) {
	l.logger.Error(fmt.Sprint(args...), l.AppendCustomField(nil)...)
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
	msg := "success"
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

func (l *ormLogger) AppendCustomField(ctx context.Context) []zlog.Field {
	var requestID string
	if c, ok := ctx.(*gin.Context); ok && c != nil {
		requestID, _ = ctx.Value(zlog.ContextKeyRequestID).(string)
	}
	fields := []zlog.Field{
		zlog.String("requestId", requestID),
	}
	return fields
}
