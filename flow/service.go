package flow

import "fmt"

// Deprecated: define business behavior in the consuming package. This marker
// interface does not provide a framework extension point.
type IService interface {
	ILayer
	ServiceFunc()
}

// Deprecated: define a concrete business type in the consuming package.
type Service struct {
	Layer
}

func (entity *Service) ServiceFunc() {
	fmt.Print("this is service func\n")
}
