package flow

import "fmt"

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
