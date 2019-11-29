package core

import (
	"github.com/gomodule/redigo/redis"
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
	return false
}

// 超时未回复 关闭自动抢单
func (dao *Dao) TimeOutGrabOrder(userId int64, exit chan struct{}) {
	c := dao.Cpool.Get()
	defer c.Close()
	// keyQuickOrder := RKGameQuickOrder()

	// var timeOut time.Duration
	// timeOut, _ = redis.Int64(c.Do("HGET", keyQuickOrder, "chat_timeout"))

	// d := time.Duration(1 * time.Minute)
	// counts := int64(65)
	ticker := time.NewTimer(time.Minute * 50)
	defer ticker.Stop()
	key := RKChatTimes(userId)
	c.Do("set", key, 1)
GL:
	for {
		select {
		case <-ticker.C:
			tag, _ := redis.Int64(c.Do("get", key))
			if tag == 2 {
				dao.PhpHttps(userId, 1)
			}
		case <-exit:
			break GL
		}
	}
}

func (dao *Dao) DelGodInfoCache(godID, gameID int64) {
	c := dao.Cpool.Get()
	defer c.Close()
	c.Do("DEL", RKOneGodGameV1(godID, gameID))
}

// 关闭自动抢单功能
func (dao *Dao) CloseAutoGrabOrder(godID, gameID int64) {
	c := dao.Cpool.Get()
	defer c.Close()
	c.Do("SREM", RKGodAutoGrabGames(godID), gameID)
}
