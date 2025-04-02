package flow

import "fmt"

// 业务分层，没有特殊逻辑
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
