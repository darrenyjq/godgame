package core

import (
	"github.com/gomodule/redigo/redis"
	"iceberg/frame/icelog"
	"laoyuegou.pb/godgame/model"
	"time"
)

// 更新大神急速接单开关
func (dao *Dao) AcceptQuickOrderSetting(userId, gameId, setting int64) error {
	err := dao.dbw.Table("play_god_accept_setting").Where("god_id=? AND game_id=?", userId, gameId).
		Update("grab_switch5", setting).Error
	if err != nil {
		return err
	}
	redisConn := dao.Cpool.Get()
	redisKey := GodAcceptOrderSettingKey(userId)
	redisConn.Do("DEL", redisKey)
	return nil
}

// 获取全部 急速接单大神ids
func (dao *Dao) GetQuickOrderGods() (data []model.ORMOrderAcceptSetting, err error) {
	err = dao.dbw.Table("play_god_accept_setting").
		Select("god_id,game_id").
		Where("grab_switch5=? AND grab_switch=?", 1, 1).Find(&data).Error

	if err != nil {
		return data, err
	}
	return data, nil

}

// 获取 急速接单大神所有品类
func (dao *Dao) GetAcceptSettings(godId int64) (data []model.ORMOrderAcceptSetting, err error) {
	err = dao.dbw.Table("play_god_accept_setting").
		Select("god_id,game_id").
		Where("grab_switch5=? AND grab_switch=? and god_id=?", 1, 1, godId).Find(&data).Error
	if err != nil {
		return data, err
	}
	return data, nil

}

// 是否为 抢单大神的对话  私聊用的
func (dao *Dao) GetGrabBedGodsOfBoss(userIds []int64) bool {
	c := dao.Cpool.Get()
	defer c.Close()
	key := RKGrabBedGodsOfBoss(userIds[0])
	re, err := redis.Bool(c.Do("sismember", key, userIds[1]))
	if err == nil && re {
		return true
	}

	key = RKGrabBedGodsOfBoss(userIds[1])
	re, err = redis.Bool(c.Do("sismember", key, userIds[0]))
	if err == nil && re {
		return true
	}

	return false
}

// 超时未回复 关闭自动抢单
func (dao *Dao) TimeOutGrabOrder(GodId int64) {
	c := dao.Cpool.Get()
	defer c.Close()
	// keyQuickOrder := RKQuickOrder()
	// timeOut, _ := redis.Int64(c.Do("HGET", keyQuickOrder, "chat_timeout"))
	// ticker := time.NewTimer(time.Minute * time.Duration(timeOut))
	ticker := time.NewTimer(time.Second * 60)
	defer ticker.Stop()
	key := RKChatTimes(GodId)
	c.Do("setex", key, 60, 1)
	for {
		select {
		case <-ticker.C:
			tag, _ := redis.Int64(c.Do("get", key))
			if tag == 1 {
				icelog.Info("超时未回复 通知php")
				c.Do("del", key)
				dao.PhpHttps(GodId, 1)
			}
		}
	}
}

// 离线1小时候自动 关闭抢单
func (dao *Dao) OffLineTimer(userId int64) {
	c := dao.Cpool.Get()
	defer c.Close()
	m, _ := redis.Int64(c.Do("hget", RKQuickOrder(), "off_line_time"))
	lastTime := time.Now().Unix()
	c.Do("set", RKOffLineTime(userId), lastTime)
	ticker := time.NewTimer(time.Minute * time.Duration(m))
	defer ticker.Stop()
	select {
	case <-ticker.C:
		lts, _ := redis.Int64(c.Do("get", RKOffLineTime(userId)))
		now := time.Now().Unix()
		diff := now - lts
		// icelog.Info("大神离线通知xiaxian!!!!", now, lts, m, diff)
		if diff > 60*m && now != diff {
			icelog.Info("大神离线通知php ，关闭自动接单", userId)
			dao.PhpHttps(userId, 2)
		}
	}
}

func (dao *Dao) DelGodInfoCache(godID, gameID int64) {
	c := dao.Cpool.Get()
	defer c.Close()
	c.Do("DEL", RKOneGodGameV1(godID, gameID))
}

func (dao *Dao) DelOffLineTime(godID int64) {
	c := dao.Cpool.Get()
	defer c.Close()
	c.Do("DEL", RKOffLineTime(godID))
}

// 关闭自动抢单功能
func (dao *Dao) CloseAutoGrabOrder(godID, gameID int64) {
	c := dao.Cpool.Get()
	defer c.Close()
	c.Do("SREM", RKGodAutoGrabGames(godID), gameID)
}
