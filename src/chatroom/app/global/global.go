package global

import (
	"chatroom/app"
	"chatroom/socketServer"
	"time"
)

var (
	RecordManger = app.NewRecordManger(50, 5*time.Second)
	Users        = map[int64]*app.TUser{} //sync.Map
	DfaFilter    *app.TDfaFilter
	Server       *socketServer.TSocketServer
)

func init() {
	//TODO:dfaFilter添加数据列表
}
