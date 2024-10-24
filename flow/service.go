package flow

import "fmt"

type IService interface {
	ILayer
	ServiceFunc()
}

type Service struct {
	Layer
}

func (entity *Service) ServiceFunc() {
	fmt.Print("this is service func\n")
}
