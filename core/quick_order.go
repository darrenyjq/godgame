package core

import (
	"github.com/gomodule/redigo/redis"
	"laoyuegou.pb/godgame/model"
	"time"
)

func (dao *Dao) AcceptQuickOrderSetting(userId, gameId, setting int64) error {
	err := dao.dbw.Table("play_god_accept_setting").Where("god_id=? AND game_id=?", userId, gameId).
		Update("grab_switch5", setting).Error
	if err != nil {
		return err
	}
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

func (dao *Dao) GetGrabBedGodsOfBoss(userIds []int64) {
	c := dao.cpool.Get()
	defer c.Close()
	if len(userIds) == 2 {
		key := RKGrabBedGodsOfBoss(userIds[0])
		re, err := redis.Bool(c.Do("sismember", key, userIds[1]))
		if err == nil && re {

		}
		key = RKGrabBedGodsOfBoss(userIds[0])
		re, err = redis.Bool(c.Do("sismember", key, userIds[0]))
		if err == nil && re {

		}
	}

	return
}

// 倒计时抢单开关
func (dao *Dao) GrabOrderLoop(userId int64, exit chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	counts := 65
	c := dao.cpool.Get()
	key := RKChatTimes(userId)
GL:
	for {
		select {
		case <-ticker.C:
			c.Do("get", key)
			if counts < 0 {

			}
			counts--
		case <-exit:
			break GL
		}
	}
}

func (dao *Dao) DelGodInfoCache(godID, gameID int64) {
	c := dao.cpool.Get()
	defer c.Close()
	c.Do("DEL", RKOneGodGameV1(godID, gameID))
}
