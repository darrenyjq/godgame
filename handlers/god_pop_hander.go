package handlers

import (
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"godgame/core"
	"iceberg/frame/icelog"
	"laoyuegou.pb/imcourier/pb"
	"time"
)

type GodImOnline struct {
	dao *core.Dao
}

// 监听大神上下线事件 记录到ES中
func (self *GodImOnline) HandleMessage(msg *nsq.Message) error {
	var message imcourierpb.IMEventMsg
	err := proto.Unmarshal(msg.Body, &message)
	if err != nil {
		icelog.Error(err.Error())
		return err
	}
	if message.Event == imcourierpb.IMEvent_IMEventOnline {
		self.esUpdate(message.ClientInfo.ClientId, fmt.Sprintf("%s", "lts"))

	}
	if message.Event == imcourierpb.IMEvent_IMEventOffline {
		// icelog.Info("离线了！！！！！", message)
		// self.esQueryQuickOrder(message.ClientInfo.ClientId, fmt.Sprintf("%s", "offlinetime"))
		if message.ClientInfo.ClientId > 0 {
			go self.dao.OffLineTimer(message.ClientInfo.ClientId)
		}
	}
	return nil
}

// 查询大神池 更新es
func (self *GodImOnline) esUpdate(godId int64, lineTime string) {
	data := self.dao.EsQueryQuickOrder(godId)
	self.dao.DelOffLineTime(godId)
	if len(data) > 0 {
		for _, item := range data {
			self.dao.EsUpdateQuickOrder(item.Id, map[string]interface{}{
				lineTime: time.Now(),
			})
		}
	}
}
