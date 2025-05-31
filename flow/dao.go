package flow

import (
	"errors"
	"fmt"
	errors2 "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/zlog"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"time"
)

const (
	ctxKeyReadDbMaster = "__isReadDbMaster__"
)

var (
	DefaultDBClient *gorm.DB
	NamedDBClient   map[string]*gorm.DB
)

type IDao interface {
	ILayer
	GetDB() *gorm.DB
	GetDBByName(name string) *gorm.DB
	SetDB(db *gorm.DB)
	ResetDB()
	ClearDB()
	SetTable(tableName string)
	GetTable() string
	SetReadDbMaster(isReadMaster bool)
	GetReadDbMaster() bool
}

type Dao struct {
	Layer
	db           *gorm.DB
	defaultDB    *gorm.DB
	tableName    string
	partitionNum int
}

func (d *Dao) OnCreate() {
	// hook if needed
}

func (d *Dao) getDBBase(db *gorm.DB) *gorm.DB {
	if db == nil {
		return nil
	}
	if d.tableName != "" {
		return db.WithContext(d.GetCtx()).Table(d.tableName)
	}
	return db.WithContext(d.GetCtx())
}

// GetDB 优先返回 entity.db, 否则 defaultDB, 否则 DefaultDBClient
func (d *Dao) GetDB() *gorm.DB {
	if d.db != nil {
		return d.getDBBase(d.db)
	}
	if d.defaultDB != nil {
		return d.getDBBase(d.defaultDB)
	}
	return d.getDBBase(DefaultDBClient)
}

// GetDBByName 支持根据名称获取对应 DB，名称为空返回默认 DB
func (d *Dao) GetDBByName(name string) *gorm.DB {
	if d.db != nil {
		return d.getDBBase(d.db)
	}
	if name == "" {
		return d.getDBBase(DefaultDBClient)
	}
	if NamedDBClient != nil {
		if dbClient, ok := NamedDBClient[name]; ok {
			return d.getDBBase(dbClient)
		}
	}
	return nil
}

func (d *Dao) SetDB(db *gorm.DB) {
	d.db = db
}

func (d *Dao) SetDefaultDB(db *gorm.DB) {
	d.defaultDB = db
}

func (d *Dao) ResetDB() {
	if d.defaultDB != nil {
		d.db = d.defaultDB.WithContext(d.GetCtx())
	} else if DefaultDBClient != nil {
		d.db = DefaultDBClient.WithContext(d.GetCtx())
	} else {
		d.db = nil
	}
}

func (d *Dao) ClearDB() {
	d.db = nil
}

func (d *Dao) SetTable(tableName string) {
	d.tableName = tableName
}

func (d *Dao) GetTable() string {
	return d.tableName
}

func (d *Dao) SetPartitionNum(num int) {
	if num < 0 {
		num = 0
	}
	d.partitionNum = num
}

func (d *Dao) GetPartitionNum() int {
	return d.partitionNum
}

func (d *Dao) SetReadDbMaster(isReadMaster bool) {
	d.ctx.Set(ctxKeyReadDbMaster, isReadMaster)
}

func (d *Dao) GetReadDbMaster() bool {
	v, exist := d.ctx.Get(ctxKeyReadDbMaster)
	if !exist {
		return false
	}
	is, ok := v.(bool)
	return ok && is
}

// 计算分表名称，防止分区数量为 0 导致 panic
func (d *Dao) GetPartitionTable(value int64) string {
	if d.partitionNum <= 0 {
		return d.GetTable()
	}
	return fmt.Sprintf("%s%d", d.GetTable(), value%int64(d.partitionNum))
}

func SetDefaultDBClient(db *gorm.DB) {
	DefaultDBClient = db
}

func SetNamedDBClient(namedDbs map[string]*gorm.DB) {
	NamedDBClient = namedDbs
}

type CommonDao[T schema.Tabler] struct {
	Dao
}

func (c *CommonDao[T]) Insert(add *T) error {
	if add == nil {
		return nil
	}
	if err := c.GetDB().Create(add).Error; err != nil {
		zlog.Error(c.GetCtx(), "CommonDao.Insert error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) Update(update *T) error {
	if update == nil {
		return errors.New("update entity cannot be nil")
	}
	if err := c.GetDB().Save(update).Error; err != nil {
		zlog.Error(c.GetCtx(), "CommonDao.Update error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) Delete(delete *T) error {
	if delete == nil {
		return errors.New("delete entity cannot be nil")
	}
	if err := c.GetDB().Delete(delete).Error; err != nil {
		zlog.Error(c.GetCtx(), "CommonDao.Delete error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) BatchInsert(add []*T) error {
	if len(add) == 0 {
		return nil
	}
	const batchSize = 2000
	if err := c.GetDB().CreateInBatches(add, batchSize).Error; err != nil {
		zlog.Error(c.GetCtx(), "CommonDao.BatchInsert error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) UpdateById(id any, update map[string]interface{}) error {
	if update == nil {
		return errors.New("update map cannot be nil")
	}
	update["updated_at"] = time.Now()
	var t T
	db := c.GetDB().Model(&t)
	if err := db.Where("id = ?", id).Updates(update).Error; err != nil {
		zlog.Error(c.GetCtx(), "CommonDao.UpdateById error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) GetById(id any) (*T, error) {
	var res T
	err := c.GetDB().Where("id = ?", id).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		zlog.Error(c.GetCtx(), "CommonDao.GetById error: %v", err)
		return nil, errors2.ErrorSystemError
	}
	return &res, nil
}

func (c *CommonDao[T]) DeleteById(id any) error {
	var t T
	if err := c.GetDB().Where("id = ?", id).Delete(&t).Error; err != nil {
		zlog.Error(c.GetCtx(), "CommonDao.DeleteById error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}
