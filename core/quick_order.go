package core

func (dao *Dao) AcceptQuickOrderSetting(userId, gameId, setting int64) error {
	err := dao.dbw.Table("play_god_accept_setting").Where("god_id=? AND game_id=?", userId, gameId).
		Update("grab_switch5", setting).Error
	if err != nil {
		return err
	}

	// redisConn := dao.cpool.Get()
	// redisConn.Do("DEL", GodAcceptOrderSettingKey(settings.GodID), RKOneGodGameV1(settings.GodID, settings.GameID), RKSimpleGodGamesKey(settings.GodID))
	// redisConn.Close()
	return nil
}
