package flow

import (
	"context"
	"errors"
	"fmt"
	"time"

	errors2 "github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/zlog"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var ErrDatabaseNotConfigured = errors.New("flow: database is not configured")

type DBRegistry struct {
	defaultDB *gorm.DB
	named     map[string]*gorm.DB
}

func NewDBRegistry(defaultDB *gorm.DB, named map[string]*gorm.DB) *DBRegistry {
	registry := &DBRegistry{defaultDB: defaultDB, named: make(map[string]*gorm.DB, len(named))}
	for name, db := range named {
		registry.named[name] = db
	}
	return registry
}

func (registry *DBRegistry) Default() *gorm.DB {
	if registry == nil {
		return nil
	}
	return registry.defaultDB
}

func (registry *DBRegistry) Get(name string) *gorm.DB {
	if registry == nil {
		return nil
	}
	if name == "" {
		return registry.defaultDB
	}
	return registry.named[name]
}

// Deprecated: compose Dao directly and define small interfaces in the
// consuming package. This broad interface is kept for source compatibility.
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
	registry     *DBRegistry
	tableName    string
	partitionNum int
	readMaster   bool
}

func NewDao(registry *DBRegistry) Dao {
	return Dao{registry: registry}
}

func (d *Dao) OnCreate() {}

func (d *Dao) getDBBase(db *gorm.DB) *gorm.DB {
	return d.getDBBaseContext(d.GetCtx(), db)
}

func (d *Dao) getDBBaseContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	if db == nil {
		return nil
	}
	if d.tableName != "" {
		return db.WithContext(ctx).Table(d.tableName)
	}
	return db.WithContext(ctx)
}

func (d *Dao) GetDB() *gorm.DB {
	return d.GetDBContext(d.GetCtx())
}

func (d *Dao) GetDBContext(ctx context.Context) *gorm.DB {
	if d.db != nil {
		return d.getDBBaseContext(ctx, d.db)
	}
	if d.registry == nil {
		return nil
	}
	return d.getDBBaseContext(ctx, d.registry.Default())
}

func (d *Dao) GetDBByName(name string) *gorm.DB {
	return d.GetDBByNameContext(d.GetCtx(), name)
}

func (d *Dao) GetDBByNameContext(ctx context.Context, name string) *gorm.DB {
	if d.db != nil {
		return d.getDBBaseContext(ctx, d.db)
	}
	if d.registry == nil {
		return nil
	}
	return d.getDBBaseContext(ctx, d.registry.Get(name))
}

func (d *Dao) SetDB(db *gorm.DB) {
	d.db = db
}

func (d *Dao) SetRegistry(registry *DBRegistry) {
	d.registry = registry
}

func (d *Dao) ResetDB() {
	d.db = nil
}

func (d *Dao) ClearDB() {
	d.ResetDB()
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
	d.readMaster = isReadMaster
}

func (d *Dao) GetReadDbMaster() bool {
	return d.readMaster
}

// 计算分表名称，防止分区数量为 0 导致 panic
func (d *Dao) GetPartitionTable(value int64) string {
	if d.partitionNum <= 0 {
		return d.GetTable()
	}
	return fmt.Sprintf("%s%d", d.GetTable(), value%int64(d.partitionNum))
}

type CommonDao[T schema.Tabler] struct {
	Dao
}

func (c *CommonDao[T]) requireDB() (*gorm.DB, error) {
	return c.requireDBContext(c.GetCtx())
}

func (c *CommonDao[T]) requireDBContext(ctx context.Context) (*gorm.DB, error) {
	db := c.GetDBContext(ctx)
	if db == nil {
		zlog.Error(ctx, ErrDatabaseNotConfigured)
		return nil, ErrDatabaseNotConfigured
	}
	return db, nil
}

func (c *CommonDao[T]) Insert(add *T) error {
	return c.InsertContext(c.GetCtx(), add)
}

func (c *CommonDao[T]) InsertContext(ctx context.Context, add *T) error {
	if add == nil {
		return nil
	}
	db, err := c.requireDBContext(ctx)
	if err != nil {
		return err
	}
	if err = db.Create(add).Error; err != nil {
		zlog.Error(ctx, "CommonDao.Insert error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) Update(update *T) error {
	return c.UpdateContext(c.GetCtx(), update)
}

func (c *CommonDao[T]) UpdateContext(ctx context.Context, update *T) error {
	if update == nil {
		return errors.New("update entity cannot be nil")
	}
	db, err := c.requireDBContext(ctx)
	if err != nil {
		return err
	}
	if err = db.Save(update).Error; err != nil {
		zlog.Error(ctx, "CommonDao.Update error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) Delete(delete *T) error {
	return c.DeleteContext(c.GetCtx(), delete)
}

func (c *CommonDao[T]) DeleteContext(ctx context.Context, delete *T) error {
	if delete == nil {
		return errors.New("delete entity cannot be nil")
	}
	db, err := c.requireDBContext(ctx)
	if err != nil {
		return err
	}
	if err = db.Delete(delete).Error; err != nil {
		zlog.Error(ctx, "CommonDao.Delete error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) BatchInsert(add []*T) error {
	return c.BatchInsertContext(c.GetCtx(), add)
}

func (c *CommonDao[T]) BatchInsertContext(ctx context.Context, add []*T) error {
	if len(add) == 0 {
		return nil
	}
	db, err := c.requireDBContext(ctx)
	if err != nil {
		return err
	}
	const batchSize = 2000
	if err = db.CreateInBatches(add, batchSize).Error; err != nil {
		zlog.Error(ctx, "CommonDao.BatchInsert error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) UpdateById(id any, update map[string]interface{}) error {
	return c.UpdateByIDContext(c.GetCtx(), id, update)
}

func (c *CommonDao[T]) UpdateByIDContext(ctx context.Context, id any, update map[string]interface{}) error {
	if update == nil {
		return errors.New("update map cannot be nil")
	}
	updates := make(map[string]interface{}, len(update)+1)
	for field, value := range update {
		updates[field] = value
	}
	updates["updated_at"] = time.Now()
	database, err := c.requireDBContext(ctx)
	if err != nil {
		return err
	}
	var t T
	db := database.Model(&t)
	if err = db.Where("id = ?", id).Updates(updates).Error; err != nil {
		zlog.Error(ctx, "CommonDao.UpdateById error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) GetById(id any) (*T, error) {
	return c.GetByIDContext(c.GetCtx(), id)
}

func (c *CommonDao[T]) GetByIDContext(ctx context.Context, id any) (*T, error) {
	db, err := c.requireDBContext(ctx)
	if err != nil {
		return nil, err
	}
	var res T
	err = db.Where("id = ?", id).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		zlog.Error(ctx, "CommonDao.GetById error: %v", err)
		return nil, errors2.ErrorSystemError
	}
	return &res, nil
}

func (c *CommonDao[T]) DeleteById(id any) error {
	return c.DeleteByIDContext(c.GetCtx(), id)
}

func (c *CommonDao[T]) DeleteByIDContext(ctx context.Context, id any) error {
	db, err := c.requireDBContext(ctx)
	if err != nil {
		return err
	}
	var t T
	if err = db.Where("id = ?", id).Delete(&t).Error; err != nil {
		zlog.Error(ctx, "CommonDao.DeleteById error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}
