# 工具函数集合

提供常用的工具函数，包括通道操作、字符串处理等实用功能。

## 功能特性

- ✅ **安全通道操作**: 提供安全的通道发送功能，避免向已关闭通道发送数据
- ✅ **轻量级设计**: 简洁的API设计，易于使用和集成
- ✅ **错误处理**: 优雅处理各种边界情况

## 通道工具函数

### SafeSendBool

安全地向布尔通道发送数据，避免向已关闭的通道发送数据时产生panic。

```go
func SafeSendBool(ch chan bool, value bool) (closed bool)
```

**参数:**
- `ch chan bool`: 目标布尔通道
- `value bool`: 要发送的布尔值

**返回值:**
- `closed bool`: 如果通道已关闭返回 true，否则返回 false

## 使用示例

### 基本使用

```go
package main

import (
    "fmt"
    "time"
    "github.com/xiangtao94/golib/pkg/utils"
)

func main() {
    // 创建一个布尔通道
    ch := make(chan bool, 1)
    
    // 安全发送数据
    closed := utils.SafeSendBool(ch, true)
    if closed {
        fmt.Println("通道已关闭")
    } else {
        fmt.Println("数据发送成功")
    }
    
    // 接收数据
    value := <-ch
    fmt.Printf("接收到值: %v\n", value)
}
```

### 并发场景下的安全使用

```go
package main

import (
    "fmt"
    "sync"
    "time"
    "github.com/xiangtao94/golib/pkg/utils"
)

func main() {
    ch := make(chan bool, 1)
    var wg sync.WaitGroup
    
    // 启动一个goroutine来关闭通道
    wg.Add(1)
    go func() {
        defer wg.Done()
        time.Sleep(100 * time.Millisecond)
        close(ch)
        fmt.Println("通道已关闭")
    }()
    
    // 启动多个goroutine尝试发送数据
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            time.Sleep(time.Duration(id*50) * time.Millisecond)
            
            closed := utils.SafeSendBool(ch, true)
            if closed {
                fmt.Printf("Goroutine %d: 检测到通道已关闭\n", id)
            } else {
                fmt.Printf("Goroutine %d: 数据发送成功\n", id)
            }
        }(i)
    }
    
    wg.Wait()
}
```

### 任务完成通知模式

```go
package main

import (
    "fmt"
    "sync"
    "time"
    "github.com/xiangtao94/golib/pkg/utils"
)

type TaskManager struct {
    done    chan bool
    workers sync.WaitGroup
}

func NewTaskManager() *TaskManager {
    return &TaskManager{
        done: make(chan bool, 1),
    }
}

func (tm *TaskManager) AddTask(taskFunc func()) {
    tm.workers.Add(1)
    go func() {
        defer tm.workers.Done()
        taskFunc()
        
        // 安全地发送完成信号
        closed := utils.SafeSendBool(tm.done, true)
        if !closed {
            fmt.Println("任务完成信号已发送")
        }
    }()
}

func (tm *TaskManager) Wait() {
    go func() {
        tm.workers.Wait()
        close(tm.done)
    }()
    
    // 等待至少一个任务完成或所有任务完成
    <-tm.done
    fmt.Println("收到任务完成通知")
}

func main() {
    tm := NewTaskManager()
    
    // 添加多个任务
    for i := 0; i < 3; i++ {
        taskID := i
        tm.AddTask(func() {
            time.Sleep(time.Duration(taskID+1) * time.Second)
            fmt.Printf("任务 %d 完成\n", taskID)
        })
    }
    
    tm.Wait()
}
```

### 超时控制模式

```go
package main

import (
    "fmt"
    "time"
    "context"
    "github.com/xiangtao94/golib/pkg/utils"
)

func processWithTimeout(ctx context.Context, timeout time.Duration) {
    done := make(chan bool, 1)
    
    // 启动处理任务
    go func() {
        // 模拟耗时操作
        time.Sleep(2 * time.Second)
        
        // 安全发送完成信号
        closed := utils.SafeSendBool(done, true)
        if !closed {
            fmt.Println("处理完成")
        }
    }()
    
    // 超时控制
    select {
    case <-done:
        fmt.Println("任务正常完成")
    case <-time.After(timeout):
        fmt.Println("任务超时")
        close(done) // 关闭通道，避免goroutine泄露
    case <-ctx.Done():
        fmt.Println("任务被取消")
        close(done)
    }
}

func main() {
    ctx := context.Background()
    
    fmt.Println("测试正常完成:")
    processWithTimeout(ctx, 3*time.Second)
    
    fmt.Println("\n测试超时:")
    processWithTimeout(ctx, 1*time.Second)
}
```

## API 参考

### SafeSendBool

```go
func SafeSendBool(ch chan bool, value bool) (closed bool)
```

安全地向布尔通道发送数据。

**工作原理:**
1. 使用 `defer` 和 `recover()` 捕获可能的 panic
2. 如果通道已关闭，向其发送数据会触发 panic
3. 捕获到 panic 时返回 `true` 表示通道已关闭
4. 正常发送成功时返回 `false`

**使用场景:**
- 并发环境下不确定通道状态时的安全发送
- 避免因向已关闭通道发送数据而导致程序崩溃
- 需要检测通道是否已关闭的场景

**注意事项:**
- 这个函数只适用于 `chan bool` 类型的通道
- 函数会阻塞直到数据发送成功或检测到通道已关闭
- 如果通道缓冲区已满且通道未关闭，函数会一直阻塞

## 扩展计划

未来版本可能会添加更多工具函数：

- 泛型版本的安全通道发送函数
- 字符串处理工具
- 数组/切片操作工具
- 时间处理工具
- 加密/解密工具等

## 贡献指南

如果您有常用的工具函数想要添加到这个包中，请确保：

1. 函数功能单一且通用
2. 提供完整的文档和使用示例
3. 包含适当的错误处理
4. 遵循Go语言的最佳实践 
 