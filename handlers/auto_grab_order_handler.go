package handlers

import (
	"github.com/gomodule/redigo/redis"
	"github.com/nsqio/go-nsq"
	"godgame/core"
	"iceberg/frame/icelog"
	"laoyuegou.pb/imapi/pb"
	"time"
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
	var user1, user2 int64
	user1 = message.S
	user2 = message.R[0]
	Rkey := core.RkSendImStartTime()
	key := core.RKChatTimes(user1, user2)
	key2 := core.RKChatTimes(user2, user1)
	c := self.dao.Cpool.Get()
	defer c.Close()
	tag, _ = redis.Int64(c.Do("Get", key))
	tag2, _ := redis.Int64(c.Do("Get", key2))

	// tag ==2 表示大神回复了老板
	if tag2 > 0 {
		c.Do("zrem", Rkey, user1)
		// 删除后延时 300s 防止被锁
		c.Do("setex", key2, 300, 2)
		return nil
	}

	// 接收者为大神
	if self.dao.IsGrabBedGodsOfBoss([]int64{user1, user2}) {
		// 查接收者是否有开启自动接单
		arr, _ := redis.Int64s(c.Do("scan", core.RKGodAutoGrabGames(user2)))
		// tag2==0表示 接受者没有发过消息   tag！=1是发消息人第一次发
		if tag2 == 0 && len(arr) > 0 && tag == 0 {
			c.Do("zadd", Rkey, time.Now().Unix(), user2)
			// tag ==1 表示老板第一次找大神
			c.Do("setex", key, 300, 1)
			return nil
		}
		return nil
	}
	return nil
}
