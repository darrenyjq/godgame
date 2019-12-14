package core

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"iceberg/frame/icelog"
	"iceberg/frame/protocol"
	"laoyuegou.com/http_api"
	"time"
)

// 上下线消息事件
func (dao *Dao) StartGodLineLoop() {
	ticker := time.NewTicker(time.Minute * time.Duration(5))
	// ticker := time.NewTicker(time.Second * time.Duration(15))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			dao.handleOffLineTimer()
		case <-dao.ExitChatChan:
			goto exit
		}
	}
exit:
	icelog.Info("exiting StartGodLine loop...")
}

// 离线1小时候自动 关闭抢单
func (dao *Dao) handleOffLineTimer() {
	c := dao.Cpool.Get()
	defer c.Close()
	T, _ := redis.Int64(c.Do("hget", RKQuickOrder(), "off_line_time"))
	if T == 0 {
		T = 15
	}
	Rkey := RkGodOfflineTime()
	// 规定时间范围 以前
	endTime := time.Now().Unix() - T*60
	arr, _ := redis.Int64s(c.Do("ZRANGEBYSCORE", Rkey, 0, endTime, "WITHSCORES"))
	userIds := ""
	for _, userId := range arr {
		userIds += fmt.Sprintf(",%d", userId)
	}
	if len(userIds) > 0 {
		userIds = userIds[0 : len(userIds)-1]
		dao.PhpHttps(userIds, 2)
		c.Do("ZREMRANGEBYSCORE", Rkey, 0, endTime)
	}
	icelog.Info("离线1小时候自动 打印关闭抢单开关", userIds, Rkey, endTime)
}

// 消息超时监听
func (dao *Dao) StartImLoop() {
	ticker := time.NewTicker(time.Minute * time.Duration(5))
	// ticker := time.NewTicker(time.Second * time.Duration(15))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			dao.ChatTimeOut()
		case <-dao.ExitImChan:
			goto exit
		}
	}

exit:
	icelog.Info("exiting StartImLoop loop...")
}

// 超时未回复 关闭自动抢单
func (dao *Dao) ChatTimeOut() {
	c := dao.Cpool.Get()
	defer c.Close()
	T, _ := redis.Int64(c.Do("hget", RKQuickOrder(), "chat_timeout"))
	if T == 0 {
		T = 15
	}
	Rkey := RkSendImStartTime()
	// 规定时间范围 以前
	endTime := time.Now().Unix() - T*60
	arr, _ := redis.Int64s(c.Do("ZRANGEBYSCORE", Rkey, 0, endTime, "WITHSCORES"))
	userIds := ""
	for _, userId := range arr {
		userIds += fmt.Sprintf(",%d", userId)
	}
	if len(userIds) > 0 {
		userIds = userIds[0 : len(userIds)-1]
		dao.PhpHttps(userIds, 1)
		c.Do("ZREMRANGEBYSCORE", Rkey, 0, endTime)
	}
	icelog.Info("超时未回复 打印关闭抢单开关", userIds, Rkey, endTime)

}

// ReloadImLoop 重加载信号
func (dao *Dao) ReloadChanLoop(signal int64) {
	c := dao.Cpool.Get()
	key := RkLockProtect(signal)
	lock, _ := redis.Int64(c.Do("Get", key))
	if lock > 0 {
		return
	}
	switch signal {
	case 1:
		close(dao.ExitImChan)
		dao.ExitImChan = make(chan int, 1)
		go dao.StartImLoop()
		icelog.Info("重载私聊超时channel成功")

	case 2:
		close(dao.ExitChatChan)
		dao.ExitChatChan = make(chan int, 1)
		go dao.StartGodLineLoop()
		icelog.Info("重载上下线channel成功")
	default:
		icelog.Info("没有收到正确信号")
	}
	c.Do("setex", key, 60, lock)
}

// 调用php通用 关闭自动抢单
func (dao *Dao) PhpHttps(godIds string, reason int64) {
	client := http_api.NewClient()
	url := fmt.Sprintf("%s%s", dao.Cfg.Urls["php_api"], "order/interior/quickorder/disable-auto-grab")
	resp, err := client.POSTV2(url, map[string]interface{}{
		"god_ids": godIds,
		"reason":  reason,
	})
	if err != nil {
		icelog.Error(err.Error())
	}
	if resp.StatusCode == 200 {
		qq, _ := resp.ReadAll()
		var ress protocol.Message
		err = json.Unmarshal(qq, &ress)
		icelog.Info("超时 关闭自动抢单功能", ress.Errmsg, godIds, reason)
	}
}

// 是否是抢单大神的对话  私聊用的
func (dao *Dao) IsGrabBedGodsOfBoss(userIds []int64) bool {
	c := dao.Cpool.Get()
	defer c.Close()
	key := RKGrabBedGodsOfBoss(userIds[0])
	re, err := redis.Bool(c.Do("sismember", key, userIds[1]))
	if err == nil && re {
		return true
	}
	return false
}
