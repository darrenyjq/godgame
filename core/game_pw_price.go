package core

import (
	"iceberg/frame/icelog"
	"laoyuegou.pb/game/model"
)

// LoadGamePWPrice 获取游戏下陪玩价格
func (dao *Dao) LoadGamePWPrice(gameid int64) (price []model.PwPrice, err error) {
	err = dao.dbr.Table("play_price_peiwan").Where("game_id = ?", gameid).Find(&price).Error
	if err != nil {
		return nil, err
	}
	return price, nil
}

//
func (dao *Dao) GetGamePriceId(gameid, levelId int64) (int64, error) {
	pwPrice, err := dao.LoadGamePWPrice(gameid)
	for _, price := range pwPrice {
		if price.GodLevel == levelId {
			return price.ID, nil
		}
	}
	icelog.Error(err.Error())
	return 0, err
}
