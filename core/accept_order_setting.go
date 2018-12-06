package core

import (
	"encoding/json"
	"github.com/gomodule/redigo/redis"
	"laoyuegou.pb/godgame/model"
	"play/common/key"
)

// 获取大神所有游戏的接单设置
func (dao *Dao) GetGodAcceptOrderSettings(godID int64) ([]model.OrderAcceptSetting, error) {
	var oas []model.OrderAcceptSetting
	var err error
	var tmpOas model.OrderAcceptSetting
	redisConn := dao.cpool.Get()
	redisKey := key.GodAcceptOrderSettingKey(godID)
	defer redisConn.Close()
	if exists, _ := redis.Bool(redisConn.Do("EXISTS", redisKey)); exists {
		vals, _ := redis.ByteSlices(redisConn.Do("HVALS", redisKey))
		for _, val := range vals {
			if err = json.Unmarshal(val, &tmpOas); err == nil {
				oas = append(oas, tmpOas)
			} else {
				redisConn.Do("DEL", redisKey)
			}
		}
		return oas, nil
	}
	var ormOas []model.ORMOrderAcceptSetting
	err = dao.dbr.Where("god_id=?", godID).Find(&ormOas).Error
	if err != nil {
		return oas, err
	}
	var bs []byte
	for _, tmpOrmOas := range ormOas {
		if err = json.Unmarshal([]byte(tmpOrmOas.RegionLevel), &tmpOas); err != nil {
			continue
		}
		tmpOas.GodID = tmpOrmOas.GodID
		tmpOas.GameID = tmpOrmOas.GameID
		tmpOas.PriceID = tmpOrmOas.PriceID
		tmpOas.GrabSwitch = tmpOrmOas.GrabSwitch
		tmpOas.GrabSwitch2 = tmpOrmOas.GrabSwitch2
		tmpOas.GrabSwitch3 = tmpOrmOas.GrabSwitch3
		oas = append(oas, tmpOas)
		if bs, err = json.Marshal(tmpOas); err == nil {
			redisConn.Do("HSET", redisKey, tmpOas.GameID, string(bs))
		}
	}
	return oas, nil
}

// 获取大神指定游戏的接单设置
func (dao *Dao) GetGodSpecialAcceptOrderSetting(godID, gameID int64) (model.OrderAcceptSetting, error) {
	var oas model.OrderAcceptSetting
	var err error
	redisConn := dao.cpool.Get()
	redisKey := key.GodAcceptOrderSettingKey(godID)
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
	if err = json.Unmarshal([]byte(ormOas.RegionLevel), &oas); err != nil {
		return oas, err
	}
	oas.GodID = ormOas.GodID
	oas.GameID = ormOas.GameID
	oas.PriceID = ormOas.PriceID
	oas.GrabSwitch = ormOas.GrabSwitch
	oas.GrabSwitch2 = ormOas.GrabSwitch2
	oas.GrabSwitch3 = ormOas.GrabSwitch3
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
			"pei_wan_uniprice_id": settings.PriceID,
		}).
		FirstOrCreate(&settings).Error
	if err != nil {
		return err
	}
	redisConn := dao.cpool.Get()
	redisConn.Do("DEL", key.GodAcceptOrderSettingKey(settings.GodID), key.RKGodGameV1(settings.GodID))
	redisConn.Close()
	dao.GetGodSpecialGameV1(settings.GodID, settings.GameID)
	return nil
}
