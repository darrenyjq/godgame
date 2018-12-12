package core

import (
	"encoding/json"
	"github.com/gomodule/redigo/redis"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
)

func (dao *Dao) CreateGodLeader(godLeader model.GodLeader) (model.GodLeader, error) {
	err := dao.dbw.Create(&godLeader).Error
	return godLeader, err
}

func (dao *Dao) ModifyGodLeader(godLeader model.GodLeader) (model.GodLeader, error) {
	err := dao.dbw.Save(&godLeader).Error
	if err == nil {
		c := dao.cpool.Get()
		c.Do("DEL", RKGodLeaderInfo(godLeader.ID))
		c.Close()
	}
	return godLeader, err
}

func (dao *Dao) GetGodLeaderByIDCard(idcard string) *model.GodLeader {
	var godLeader model.GodLeader
	err := dao.dbr.Table("play_god_leaders").Where("idcard=?", idcard).First(&godLeader).Error
	if err != nil {
		return nil
	}
	return &godLeader
}

func (dao *Dao) QueryGodLeaders(args godgamepb.GodLeadersReq) ([]model.GodLeader, error) {
	var godLeaders []model.GodLeader
	err := dao.dbr.Table("play_god_leaders").Offset(args.GetOffset()).Limit(10).Find(&godLeaders).Error
	if err != nil {
		return godLeaders, err
	}
	return godLeaders, nil
}

func (dao *Dao) GetGodLeaderByID(leaderID int64) *model.GodLeader {
	var godLeader model.GodLeader
	c := dao.cpool.Get()
	defer c.Close()
	redisKey := RKGodLeaderInfo(leaderID)
	bs, _ := redis.Bytes(c.Do("GET", redisKey))
	err := json.Unmarshal(bs, &godLeader)
	if err == nil {
		return &godLeader
	}
	err = dao.dbr.Table("play_god_leaders").Where("id=?", leaderID).First(&godLeader).Error
	if err != nil {
		return nil
	}
	bs, err = json.Marshal(godLeader)
	if err == nil {
		c.Do("SET", redisKey, string(bs), "EX", 2592000)
	}
	return &godLeader
}
