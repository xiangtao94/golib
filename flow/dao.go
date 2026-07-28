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

type Dao struct {
	db           *gorm.DB
	registry     *DBRegistry
	tableName    string
	partitionNum int
}

func NewDao(registry *DBRegistry) Dao {
	return Dao{registry: registry}
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

func (d *Dao) GetDB(ctx context.Context) *gorm.DB {
	if d.db != nil {
		return d.getDBBaseContext(ctx, d.db)
	}
	if d.registry == nil {
		return nil
	}
	return d.getDBBaseContext(ctx, d.registry.Default())
}

func (d *Dao) GetDBByName(ctx context.Context, name string) *gorm.DB {
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

func (c *CommonDao[T]) requireDBContext(ctx context.Context) (*gorm.DB, error) {
	db := c.GetDB(ctx)
	if db == nil {
		zlog.Error(ctx, ErrDatabaseNotConfigured)
		return nil, ErrDatabaseNotConfigured
	}
	return db, nil
}

func (c *CommonDao[T]) Insert(ctx context.Context, add *T) error {
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

func (c *CommonDao[T]) Update(ctx context.Context, update *T) error {
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

func (c *CommonDao[T]) Delete(ctx context.Context, delete *T) error {
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

func (c *CommonDao[T]) BatchInsert(ctx context.Context, add []*T) error {
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

func (c *CommonDao[T]) UpdateByID(ctx context.Context, id any, update map[string]interface{}) error {
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
		zlog.Error(ctx, "CommonDao.UpdateByID error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}

func (c *CommonDao[T]) GetByID(ctx context.Context, id any) (*T, error) {
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
		zlog.Error(ctx, "CommonDao.GetByID error: %v", err)
		return nil, errors2.ErrorSystemError
	}
	return &res, nil
}

func (c *CommonDao[T]) DeleteByID(ctx context.Context, id any) error {
	db, err := c.requireDBContext(ctx)
	if err != nil {
		return err
	}
	var t T
	if err = db.Where("id = ?", id).Delete(&t).Error; err != nil {
		zlog.Error(ctx, "CommonDao.DeleteByID error: %v", err)
		return errors2.ErrorSystemError
	}
	return nil
}
