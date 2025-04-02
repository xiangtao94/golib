package flow

import "fmt"

// 业务分层，没有特殊逻辑
type IData interface {
	ILayer
	DataFunc()
}

type Data struct {
	Layer
}

func (entity *Data) DataFunc() {
	fmt.Print("this is data func\n")
}
