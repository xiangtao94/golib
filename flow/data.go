package flow

import "fmt"

// Deprecated: define the smallest data interface in the consuming package.
// This marker interface does not provide a framework extension point.
type IData interface {
	ILayer
	DataFunc()
}

// Deprecated: define a concrete adapter in the consuming package.
type Data struct {
	Layer
}

func (entity *Data) DataFunc() {
	fmt.Print("this is data func\n")
}
