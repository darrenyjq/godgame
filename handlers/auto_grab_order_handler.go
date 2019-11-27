package handlers

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/nsqio/go-nsq"
	"godgame/core"
	"iceberg/frame/icelog"
	"laoyuegou.pb/imapi/pb"
)

type AutoGrabOrderHandler struct {
	dao *core.Dao
}

// 监听im消息 关闭自动抢单开关
func (self *AutoGrabOrderHandler) HandleMessage(msg *nsq.Message) error {
	var message imapipb.InternalContentMessage
	err := json.Unmarshal(msg.Body, &message)
	if err != nil {
		icelog.Error(err.Error())
		return nil
	}
	//检查是否为抢单大神
	if self.dao.GetGrabBedGodsOfBoss(message.R) {
		c := self.dao.Cpool.Get()
		defer c.Close()
		key := fmt.Sprintf("IM_CHAT_TIMES:{%d}", message.R[1])
		tag, _ := redis.Int64(c.Do("Get", key))
		if tag != 1 {
			Chan := make(chan struct{})
			go self.dao.TimeOutGrabOrder(message.R[1], Chan)
		}

		return nil
	}

	return nil

}

// 超时未回复自动 关闭抢单
// func (self *AutoGrabOrderHandler) OffLineTimer2(userId int64) {
// 	c := self.dao.Cpool
// 	defer c.Close()
// 	ticker := time.NewTimer(60 * time.Second)
// 	defer ticker.Stop()
// 	select {
// 	case <-ticker.C:
// 		icelog.Info("超时未回复自动", userId)
//
// 	}
// }
