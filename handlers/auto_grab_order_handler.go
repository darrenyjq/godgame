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
	// 检查是否为抢单大神
	if self.dao.GetGrabBedGodsOfBoss(message.R) {
		c := self.dao.Cpool.Get()
		defer c.Close()
		key := fmt.Sprintf("IM_CHAT_TIMES:{%d}", message.R[1])
		tag, _ := redis.Int64(c.Do("Get", key))
		if tag != 1 && tag != 2 {
			// Chan := make(chan struct{})
			go self.dao.TimeOutGrabOrder(message.R[1])
		} else if tag == 1 {
			// 	tag == 1 表示 已记录上次
			c.Do("set", key, 2)
		}
		return nil
	}

	return nil

}
