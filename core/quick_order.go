package core

import (
	"context"
	"fmt"
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
func (dao *Dao) TimeOutGrabOrder(userId, GodId int64) {
	c := dao.Cpool.Get()
	defer c.Close()
	keyQuickOrder := RKQuickOrder()
	timeOut, _ := redis.Int64(c.Do("HGET", keyQuickOrder, "chat_timeout"))
	ticker := time.NewTimer(time.Minute * time.Duration(timeOut))
	// ticker := time.NewTimer(time.Second * 10)
	defer ticker.Stop()
	key := RKChatTimes(userId, GodId)
	c.Do("setex", key, 300, 1)
	for {
		select {
		case <-ticker.C:
			res_id, _ := redis.Int64(c.Do("get", key))
			if res_id == 1 {
				// icelog.Info("超时未回复 通知php", userId, GodId)
				dao.PhpHttps(GodId, 1)
			}
			c.Do("del", key)
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
	// icelog.Info("大神离线通知", lastTime, userId)
	ticker := time.NewTimer(time.Minute * time.Duration(m))
	// ticker := time.NewTimer(time.Second * 10)

	defer ticker.Stop()
	select {
	case <-ticker.C:
		lts, _ := redis.Int64(c.Do("get", RKOffLineTime(userId)))
		now := time.Now().Unix()
		diff := now - lts
		// icelog.Info("大神离线通知xiaxian!!!!", now, lts, m, diff, userId)
		// if now != diff {
		// 存在时间差且 时间差符合后台规定时间即通知php
		if now != diff && diff > 60*m-60 {
			// icelog.Info("大神离线通知php ，关闭自动接单", userId)
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

func (dao *Dao) BuildESQuickOrder(godID, gameID int64) (model.ESQuickOrder, error) {
	var result model.ESQuickOrder
	if godID == 0 || gameID == 0 {
		return result, fmt.Errorf("get god info error %d-%d", godID, gameID)
	}
	godInfo := dao.GetGod(godID)
	if godInfo.UserID != godID {
		return result, fmt.Errorf("get god info error %d-%d", godID, gameID)
	}

	godGame := dao.GetGodGame(godID, gameID)
	if godGame.UserID == 0 {
		return result, fmt.Errorf("god game not found %d-%d", godID, gameID)
	}
	GodLevel := godGame.GodLevel
	accpetOrderSetting, err := dao.GetGodSpecialAcceptOrderSetting(godID, gameID)
	if err != nil {
		return result, fmt.Errorf("price id error %d-%d %s", godID, gameID, err.Error())
	}
	Score := dao.GetGodPotentialLevel(godID, gameID)
	result.GameID = gameID
	result.GodID = godID
	result.Gender = godInfo.Gender
	result.Price = accpetOrderSetting.PriceID
	result.UpdateTime = time.Now().Unix()
	result.OnlineTime = time.Now()
	result.LevelID = accpetOrderSetting.Levels
	result.RegionID = accpetOrderSetting.Regions
	result.PotentialLevel = Score.Discounts
	result.TotalScore = Score.TotalScore
	result.Repurchase = Score.Repurchase
	result.TotalWater = Score.TotalWater
	result.TotalNumber = Score.TotalNumber
	result.GodLevel = GodLevel
	result.IsGrabOrder = accpetOrderSetting.GrabSwitch5
	return result, nil
}

// 刷新急速接单池  刷新单个大神
func (dao *Dao) FlashGodQuickOrder(god int64) {
	lists, err := dao.GetAcceptSettings(god)
	if err == nil && len(lists) > 0 {
		for _, v := range lists {
			var data model.ESQuickOrder
			data, err := dao.BuildESQuickOrder(v.GodID, v.GameID)
			if err != nil {
				continue
			}
			dao.ESAddQuickOrderInternal(data)
		}
	}
}

func (dao *Dao) ESAddQuickOrderInternal(godGame model.ESQuickOrder) error {
	icelog.Info("急速接单池添加数据", godGame.GameID, godGame.GodID)
	_, err := dao.EsClient.Index().Index(dao.Cfg.ES.PWQuickOrder).
		Type(dao.Cfg.ES.PWType).
		Id(fmt.Sprintf("%d-%d", godGame.GodID, godGame.GameID)).
		BodyJson(godGame).
		Do(context.Background())
	if err != nil {
		icelog.Errorf("ESAddQuickOrder： %+v； error： %s", godGame, err)
	}
	return nil

}

// 急速接单配置获取 是否开启自动抢单
func (dao *Dao) GetAutoGrabCfg() (int64, int64) {
	c := dao.Cpool.Get()
	keyQuickOrder := RKQuickOrder()
	re1, _ := redis.Int64(c.Do("HGET", keyQuickOrder, "is_auto_grab_order"))
	re2, _ := redis.Int64(c.Do("HGET", keyQuickOrder, "auto_grab_order_level"))

	return re1, re2
}

//  根据配置要求潜力等级 是否开启自动抢单
func (dao *Dao) GetAutoGrabCf22g(godId, gameId int64) int64 {
	c := dao.Cpool.Get()
	keyQuickOrder := RKQuickOrder()
	re, _ := redis.Int64(c.Do("HGET", keyQuickOrder, "is_auto_grab_order"))
	return re
}
