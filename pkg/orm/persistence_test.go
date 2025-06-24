package orm

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestUser 测试用户模型
type TestUser struct {
	ID        uint           `gorm:"primary_key"`
	Username  string         `gorm:"size:100;not null;unique"`
	Email     string         `gorm:"size:100;not null"`
	Age       int            `gorm:"default:0"`
	CreatedAt time.Time      `gorm:"comment:创建时间"`
	UpdatedAt time.Time      `gorm:"comment:更新时间"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:删除时间"`
}

func (TestUser) TableName() string {
	return "test_users"
}

// TestPersistenceModePure 测试纯内存模式
func TestPersistenceModePure(t *testing.T) {
	conf := MemoryConf{
		DatabaseName:    "test_pure_memory",
		PersistenceMode: PureMemo,
	}

	memDB, err := InitMemoryDB(conf)
	assert.NoError(t, err)
	assert.NotNil(t, memDB)

	db := memDB.GetDB()
	assert.NotNil(t, db)

	// 测试基本操作
	err = db.AutoMigrate(&TestUser{})
	assert.NoError(t, err)

	user := &TestUser{
		Username: "pure_memory_user",
		Email:    "pure@example.com",
		Age:      25,
	}

	err = db.Create(user).Error
	assert.NoError(t, err)
	assert.Greater(t, user.ID, uint(0))

	// 关闭数据库
	err = memDB.Close()
	assert.NoError(t, err)
}

// TestPersistenceModeFile 测试文件模式
func TestPersistenceModeFile(t *testing.T) {
	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "memory_db_test")
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "test_file.db")

	conf := MemoryConf{
		DatabaseName:    "test_file_db",
		PersistenceMode: FileMode,
		FilePath:        filePath,
	}

	memDB, err := InitMemoryDB(conf)
	assert.NoError(t, err)
	assert.NotNil(t, memDB)

	db := memDB.GetDB()
	assert.NotNil(t, db)

	// 测试数据操作
	err = db.AutoMigrate(&TestUser{})
	assert.NoError(t, err)

	user := &TestUser{
		Username: "file_user",
		Email:    "file@example.com",
		Age:      30,
	}

	err = db.Create(user).Error
	assert.NoError(t, err)
	assert.Greater(t, user.ID, uint(0))

	// 关闭数据库
	err = memDB.Close()
	assert.NoError(t, err)

	// 验证文件存在
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// 重新打开数据库，验证数据持久化
	memDB2, err := InitMemoryDB(conf)
	assert.NoError(t, err)

	db2 := memDB2.GetDB()
	var foundUser TestUser
	err = db2.First(&foundUser, user.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, user.Username, foundUser.Username)
	assert.Equal(t, user.Email, foundUser.Email)

	err = memDB2.Close()
	assert.NoError(t, err)
}

// TestPersistenceModeMemoryWithBackup 测试内存+备份模式
func TestPersistenceModeMemoryWithBackup(t *testing.T) {
	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "memory_db_backup_test")
	defer os.RemoveAll(tempDir)

	backupPath := filepath.Join(tempDir, "test_backup.db")

	conf := MemoryConf{
		DatabaseName:     "test_backup_db",
		PersistenceMode:  MemoryWithBackup,
		FilePath:         backupPath,
		BackupInterval:   1 * time.Second,
		AutoBackup:       false, // 手动备份测试
		BackupOnShutdown: true,
	}

	memDB, err := InitMemoryDB(conf)
	assert.NoError(t, err)
	assert.NotNil(t, memDB)

	db := memDB.GetDB()
	assert.NotNil(t, db)

	// 测试数据操作
	err = db.AutoMigrate(&TestUser{})
	assert.NoError(t, err)

	user := &TestUser{
		Username: "backup_user",
		Email:    "backup@example.com",
		Age:      35,
	}

	err = db.Create(user).Error
	assert.NoError(t, err)
	assert.Greater(t, user.ID, uint(0))

	// 手动备份
	err = memDB.BackupToFile(backupPath)
	assert.NoError(t, err)

	// 验证备份文件存在
	_, err = os.Stat(backupPath)
	assert.NoError(t, err)

	// 关闭数据库
	err = memDB.Close()
	assert.NoError(t, err)

	// 从备份恢复到新的内存数据库
	conf2 := MemoryConf{
		DatabaseName:    "test_restore_db",
		PersistenceMode: MemoryWithBackup,
		FilePath:        backupPath,
	}

	memDB2, err := InitMemoryDB(conf2)
	assert.NoError(t, err)

	db2 := memDB2.GetDB()
	var foundUser TestUser
	err = db2.First(&foundUser, user.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, user.Username, foundUser.Username)
	assert.Equal(t, user.Email, foundUser.Email)

	err = memDB2.Close()
	assert.NoError(t, err)
}

// TestAutoBackupFunctionality 测试自动备份功能
func TestAutoBackupFunctionality(t *testing.T) {
	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "memory_db_auto_backup_test")
	defer os.RemoveAll(tempDir)

	backupPath := filepath.Join(tempDir, "auto_backup.db")

	conf := MemoryConf{
		DatabaseName:    "test_auto_backup",
		PersistenceMode: MemoryWithBackup,
		FilePath:        backupPath,
		BackupInterval:  2 * time.Second,
		AutoBackup:      true,
	}

	memDB, err := InitMemoryDB(conf)
	assert.NoError(t, err)
	defer memDB.Close()

	db := memDB.GetDB()
	err = db.AutoMigrate(&TestUser{})
	assert.NoError(t, err)

	// 创建一些数据
	user := &TestUser{
		Username: "auto_backup_user",
		Email:    "auto@example.com",
		Age:      40,
	}

	err = db.Create(user).Error
	assert.NoError(t, err)

	// 等待自动备份
	time.Sleep(3 * time.Second)

	// 验证备份文件存在
	_, err = os.Stat(backupPath)
	assert.NoError(t, err)

	// 停止自动备份
	memDB.StopAutoBackup()

	// 创建更多数据，但不应该被自动备份
	user2 := &TestUser{
		Username: "no_backup_user",
		Email:    "nobackup@example.com",
		Age:      45,
	}
	err = db.Create(user2).Error
	assert.NoError(t, err)

	// 等待一段时间，确保没有自动备份
	time.Sleep(3 * time.Second)

	// 验证备份文件中没有第二个用户的数据
	conf2 := MemoryConf{
		DatabaseName:    "test_restore_auto",
		PersistenceMode: MemoryWithBackup,
		FilePath:        backupPath,
	}

	memDB2, err := InitMemoryDB(conf2)
	assert.NoError(t, err)
	defer memDB2.Close()

	db2 := memDB2.GetDB()
	var count int64
	err = db2.Model(&TestUser{}).Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count) // 只有第一个用户被备份了
}

// TestBackupRestore 测试手动备份和恢复
func TestBackupRestore(t *testing.T) {
	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "memory_db_manual_test")
	defer os.RemoveAll(tempDir)

	backupPath := filepath.Join(tempDir, "manual_backup.db")

	// 创建源数据库
	conf1 := MemoryConf{
		DatabaseName:    "source_db",
		PersistenceMode: PureMemo,
	}

	memDB1, err := InitMemoryDB(conf1)
	assert.NoError(t, err)

	db1 := memDB1.GetDB()
	err = db1.AutoMigrate(&TestUser{})
	assert.NoError(t, err)

	// 创建测试数据
	users := []TestUser{
		{Username: "user1", Email: "user1@example.com", Age: 20},
		{Username: "user2", Email: "user2@example.com", Age: 25},
		{Username: "user3", Email: "user3@example.com", Age: 30},
	}

	for _, user := range users {
		err = db1.Create(&user).Error
		assert.NoError(t, err)
	}

	// 手动备份
	err = memDB1.BackupToFile(backupPath)
	assert.NoError(t, err)

	err = memDB1.Close()
	assert.NoError(t, err)

	// 创建目标数据库并恢复
	conf2 := MemoryConf{
		DatabaseName:    "target_db",
		PersistenceMode: PureMemo,
	}

	memDB2, err := InitMemoryDB(conf2)
	assert.NoError(t, err)
	defer memDB2.Close()

	db2 := memDB2.GetDB()
	err = db2.AutoMigrate(&TestUser{})
	assert.NoError(t, err)

	// 从备份恢复
	err = memDB2.RestoreFromFile(backupPath)
	assert.NoError(t, err)

	// 验证数据
	var restoredUsers []TestUser
	err = db2.Find(&restoredUsers).Error
	assert.NoError(t, err)
	assert.Len(t, restoredUsers, 3)

	for i, user := range restoredUsers {
		assert.Equal(t, users[i].Username, user.Username)
		assert.Equal(t, users[i].Email, user.Email)
		assert.Equal(t, users[i].Age, user.Age)
	}
}

// TestBackwardCompatibility 测试向后兼容性
func TestBackwardCompatibility(t *testing.T) {
	conf := MemoryConf{
		DatabaseName: "compatibility_test",
	}

	// 使用旧的InitMemoryClient函数
	db, err := InitMemoryClient(conf)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// 验证基本功能
	err = db.AutoMigrate(&TestUser{})
	assert.NoError(t, err)

	user := &TestUser{
		Username: "compat_user",
		Email:    "compat@example.com",
		Age:      50,
	}

	err = db.Create(user).Error
	assert.NoError(t, err)
	assert.Greater(t, user.ID, uint(0))
}
