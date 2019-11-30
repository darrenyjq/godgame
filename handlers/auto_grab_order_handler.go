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
	icelog.Infof("******* %+v %+v 私聊消息 ！！！！！php ", message, message.R)
	var tag int64
	if message.S > 0 && len(message.R) == 1 && self.dao.GetGrabBedGodsOfBoss([]int64{message.S, message.R[0]}) {
		c := self.dao.Cpool.Get()
		defer c.Close()
		key := fmt.Sprintf("IM_CHAT_TIMES:{%d}", message.R[0])
		tag, _ = redis.Int64(c.Do("Get", key))
		icelog.Info(tag, "表设计！！！！！！！！！")

		if tag != 1 && tag != 2 {
			// Chan := make(chan struct{})
			icelog.Info("第一次问大神")
			go self.dao.TimeOutGrabOrder(message.R[0])
			return nil
		}
		key = fmt.Sprintf("IM_CHAT_TIMES:{%d}", message.S)
		tag, _ = redis.Int64(c.Do("Get", key))
		if tag == 1 {
			icelog.Info("标记一次，，已回复老板 ：2")
			// 	tag == 1 表示 已记录上次
			c.Do("setex", key, 60, 2)
		}
		return nil
	}

	return nil

}
