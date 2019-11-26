package core

import "laoyuegou.pb/godgame/model"

// 获取大神所有接单设置
func (dao *Dao) GetGodsAcceptSettings() (data []model.ORMOrderAcceptSetting, err error) {
	err = dao.dbw.Table("play_god_accept_setting").
		Select("god_id,game_id").
		Where("grab_switch=? ", 1).Find(&data).Error
	if err != nil {
		return data, err
	}
	return data, nil
}

// 获取单个大神所有接单设置
func (dao *Dao) GetGodAcceptSettings(godId int64) (data []model.ORMOrderAcceptSetting, err error) {
	err = dao.dbw.Table("play_god_accept_setting").
		Select("god_id,game_id").
		Where("grab_switch=? and god_id=?", 1, godId).Find(&data).Error
	if err != nil {
		return data, err
	}
	return data, nil

}
