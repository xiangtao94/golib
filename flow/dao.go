package flow

import (
	"fmt"
	"gorm.io/gorm"
)

// 默认db
var DefaultDBClient *gorm.DB

// 可选的db集合
var NamedDBClient map[string]*gorm.DB

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
}

type Dao struct {
	Layer
	db         *gorm.DB
	defaultDB  *gorm.DB
	tableName  string
	partionNum int
}

func (entity *Dao) OnCreate() {

}

func (entity *Dao) GetDB() *gorm.DB {
	var db *gorm.DB
	if entity.db != nil {
		db = entity.db
	} else if entity.defaultDB != nil {
		db = entity.defaultDB.WithContext(entity.GetCtx())
	} else if DefaultDBClient != nil {
		db = DefaultDBClient.WithContext(entity.GetCtx())
	}
	if db != nil {
		db = db.Table(entity.GetTable())
	}
	return db
}

func (entity *Dao) GetDBByName(name string) *gorm.DB {
	var db *gorm.DB
	if entity.db != nil {
		db = entity.db
	} else {
		// 没有name，取默认的db
		if name == "" && DefaultDBClient != nil {
			db = DefaultDBClient.WithContext(entity.GetCtx())
		} else if name != "" && NamedDBClient != nil {
			// 有name，尝试找对应的db
			if dbClient, exist := NamedDBClient[name]; exist {
				db = dbClient.WithContext(entity.GetCtx())
			}
		}
	}
	if db != nil {
		db = db.Table(entity.GetTable())
	}
	return db
}

func (entity *Dao) SetDB(db *gorm.DB) {
	entity.db = db
}

func (entity *Dao) SetDefaultDB(db *gorm.DB) {
	entity.defaultDB = db
}

func (entity *Dao) ResetDB() {
	// 优先使用entity的defaultDB
	if entity.defaultDB != nil {
		entity.db = entity.defaultDB.WithContext(entity.GetCtx())
	} else {
		entity.db = DefaultDBClient.WithContext(entity.GetCtx())
	}
}

func (entity *Dao) ClearDB() {
	entity.db = nil
}

func (entity *Dao) SetTable(tableName string) {
	entity.tableName = tableName
}

func (entity *Dao) GetTable() string {
	return entity.tableName
}

func (entity *Dao) SetPartitionNum(num int) {
	entity.partionNum = num
}

func (entity *Dao) GetPartitionNum() int {
	return entity.partionNum
}

// 标记是否需要读主库
func (entity *Dao) SetReadDbMaster(isReadMaster bool) {
	entity.ctx.Set("__isReadDbMaster__", isReadMaster)
}

func (entity *Dao) GetReadDbMaster() bool {
	if v, exist := entity.ctx.Get("__isReadDbMaster__"); exist {
		if is, ok := v.(bool); ok {
			return is
		}
	}
	return false
}

func (entity *Dao) GetPartitionTable(value int64) string {
	return fmt.Sprintf("%s%d", entity.GetTable(), value%int64(entity.partionNum))
}

func SetDefaultDBClient(db *gorm.DB) {
	DefaultDBClient = db
}

func SetNamedDBClient(namedDbs map[string]*gorm.DB) {
	NamedDBClient = namedDbs
}
