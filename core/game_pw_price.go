package core

import (
	"laoyuegou.pb/game/model"
)

// LoadGamePWPrice 获取游戏下陪玩价格
func (dao *Dao) LoadGamePWPrice(gameid, levelId int64) (priceId int64, err error) {
	var price model.PwPrice
	err = dao.dbr.Table("play_price_peiwan").Where("game_id = ? and god_level = ?", gameid, levelId).Take(&price).Error
	if err != nil {
		return 0, err
	}
	return price.ID, nil
}
