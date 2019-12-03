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
	var tag int64
	if message.S > 0 && len(message.R) == 1 && self.dao.GetGrabBedGodsOfBoss([]int64{message.S, message.R[0]}) {
		// if message.S > 0 && len(message.R) == 1 {
		var user1, user2 int64
		user1 = message.S
		user2 = message.R[0]
		// if message.S < message.R[0] {
		// 	user1 = message.S
		// 	user2 = message.R[0]
		// } else {
		// 	user1 = message.R[0]
		// 	user2 = message.S
		// }
		c := self.dao.Cpool.Get()
		defer c.Close()
		key := fmt.Sprintf("IM_CHAT_TIMES:{%d}:{%d}", user1, user2)
		tag, _ = redis.Int64(c.Do("Get", key))
		if tag != 1 && tag != 2 {
			icelog.Info("第一次问大神")
			go self.dao.TimeOutGrabOrder(user1, user2)
			return nil
		}

		key = fmt.Sprintf("IM_CHAT_TIMES:{%d}:{%d}", user2, user1)
		tag, _ = redis.Int64(c.Do("Get", key))
		// tag ==1 表示老板已经给大神发消息，待大神回复
		if tag == 1 {
			icelog.Info("第二次，大神已回复老板 ！")
			// 	tag == 1 表示 已记录上次
			c.Do("setex", key, 300, 2)
		}
		return nil
	}
	return nil

}
