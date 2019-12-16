package core

import (
	"github.com/gomodule/redigo/redis"
	"iceberg/frame/icelog"
	"laoyuegou.pb/godgame/model"
)

// 获取大神指定游戏的接单设置
func (dao *Dao) GetGodSpecialAcceptOrderSetting(godID, gameID int64) (model.OrderAcceptSetting, error) {
	var oas model.OrderAcceptSetting
	var err error
	redisConn := dao.Cpool.Get()
	redisKey := GodAcceptOrderSettingKey(godID)
	defer redisConn.Close()
	if bs, err := redis.Bytes(redisConn.Do("HGET", redisKey, gameID)); err == nil {
		err = json.Unmarshal(bs, &oas)
		if err == nil {
			return oas, nil
		}
	}
	var ormOas model.ORMOrderAcceptSetting
	err = dao.dbr.Where("god_id=? AND game_id=?", godID, gameID).First(&ormOas).Error
	if err != nil {
		return oas, err
	}
	oas.GodID = ormOas.GodID
	oas.GameID = ormOas.GameID
	oas.PriceID = ormOas.PriceID
	oas.GrabSwitch = ormOas.GrabSwitch
	oas.GrabSwitch2 = ormOas.GrabSwitch2
	oas.GrabSwitch3 = ormOas.GrabSwitch3
	oas.GrabSwitch4 = ormOas.GrabSwitch4
	oas.GrabSwitch5 = ormOas.GrabSwitch5
	if err = json.Unmarshal([]byte(ormOas.RegionLevel), &oas); err != nil {
		icelog.Error(err.Error())
	}
	if bs, err := json.Marshal(oas); err == nil {
		redisConn.Do("HSET", redisKey, gameID, string(bs))
	}
	return oas, nil
}

// 修改大神接单设置
func (dao *Dao) ModifyAcceptOrderSetting(settings model.ORMOrderAcceptSetting) error {
	err := dao.dbw.Table("play_god_accept_setting").Where("god_id=? AND game_id=?", settings.GodID, settings.GameID).
		Assign(map[string]interface{}{
			"accept_settings":     settings.RegionLevel,
			"grab_switch":         settings.GrabSwitch,
			"grab_switch2":        settings.GrabSwitch2,
			"grab_switch3":        settings.GrabSwitch3,
			"grab_switch4":        settings.GrabSwitch4,
			"pei_wan_uniprice_id": settings.PriceID,
		}).
		FirstOrCreate(&settings).Error
	if err != nil {
		return err
	}
	redisConn := dao.Cpool.Get()
	redisConn.Do("DEL", GodAcceptOrderSettingKey(settings.GodID), RKOneGodGameV1(settings.GodID, settings.GameID), RKSimpleGodGamesKey(settings.GodID))
	redisConn.Close()
	return nil
}

// 根据大神等级 获取接单价格id 修改接单设置
func (dao *Dao) UpdateAcceptOrderInfo(GodLevel, GameId, GodId int64) error {
	priceId, err := dao.LoadGamePWPrice(GameId, GodLevel)
	if err != nil {
		return err
	}
	settings := model.ORMOrderAcceptSetting{
		GameID:  GameId,
		GodID:   GodId,
		PriceID: priceId,
	}
	err = dao.dbw.Table("play_god_accept_setting").Where("god_id=? AND game_id=?", settings.GodID, settings.GameID).
		Assign(map[string]interface{}{
			"pei_wan_uniprice_id": settings.PriceID,
		}).
		FirstOrCreate(&settings).Error
	if err != nil {
		icelog.Warnf("ModifyAcceptOrderSetting error:%s", err)
		return err
	}
	redisConn := dao.Cpool.Get()
	redisConn.Do("DEL", GodAcceptOrderSettingKey(settings.GodID), RKOneGodGameV1(settings.GodID, settings.GameID), RKSimpleGodGamesKey(settings.GodID))
	defer redisConn.Close()
	return nil
}
