package handlers

import (
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"godgame/core"
	"iceberg/frame/icelog"
	"laoyuegou.com/util"
	"laoyuegou.pb/godgame/model"
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
	// icelog.Infof("########  %v", message)
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
		go self.OffLineTimer(message.ClientInfo.ClientId)
	}
	return nil
}

// 离线1小时候自动 关闭抢单
func (self *GodImOnline) OffLineTimer(userId int64) {
	ticker := time.NewTimer(3600 * time.Second)
	defer ticker.Stop()
	select {
	case <-ticker.C:
		icelog.Info("用户离线，1小时后关闭自动接单", userId)
		list := self.dao.EsQueryQuickOrder(userId)
		for _, item := range list {
			var GodInfo model.ESQuickOrder
			if err := json.Unmarshal(*item.Source, &GodInfo); err != nil {
				continue
			}
			icelog.Info(GodInfo.OnlineTime)
			formatTime, err := time.Parse("2006-01-02 15:04:05", util.XTime.String(GodInfo.OnlineTime))
			if err == nil {
				diff := time.Now().Unix() - formatTime.Unix()
				if diff > 3600 {
					// 大于1小时 自动下线
					self.dao.PhpHttps(userId, 2)
				}
			}
		}
	}
}

// 查询大神池 更新es
func (self *GodImOnline) esUpdate(godId int64, lineTime string) {
	data := self.dao.EsQueryQuickOrder(godId)
	if len(data) > 0 {
		for _, item := range data {
			self.dao.EsUpdateQuickOrder(item.Id, map[string]interface{}{
				lineTime: util.XTime(time.Now()),
			})
		}
	}
}
