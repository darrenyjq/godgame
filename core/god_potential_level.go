package core

import (
	"github.com/gomodule/redigo/redis"
	"iceberg/frame/icelog"
	"laoyuegou.pb/godgame/model"
	"time"
)

func (dao *Dao) GetGodPotentialLevel(god, gameId int64) model.StatisticsLevel {
	c := dao.Cpool.Get()
	defer c.Close()
	keyQuickOrder := RKGameQuickOrder()
	bs2, _ := redis.Bytes(c.Do("HGET", keyQuickOrder, "god_potential_level2"))
	t2 := []int{0, 0, 0, 0}
	json.Unmarshal(bs2, &t2)
	bs3, _ := redis.Bytes(c.Do("HGET", keyQuickOrder, "god_potential_level3"))
	t3 := []int{0, 0, 0, 0}
	json.Unmarshal(bs3, &t3)
	bs4, _ := redis.Bytes(c.Do("HGET", keyQuickOrder, "god_potential_level4"))
	t4 := []int{0, 0, 0, 0}
	json.Unmarshal(bs4, &t4)
	bs5, _ := redis.Bytes(c.Do("HGET", keyQuickOrder, "god_potential_level5"))
	t5 := []int{0, 0, 0, 0}
	json.Unmarshal(bs5, &t5)

	days, err := redis.Int64(c.Do("HGET", keyQuickOrder, "god_level_time_range"))
	if err == nil && days > 15 {

	} else {
		// 起码15天起
		days = 15
	}
	level := 1 // 初始默认1级
	score := dao.CalculateScore(god, gameId, days)
	if t2[0] < score.TotalWater && t2[1] < score.TotalNumber && t2[2] < score.Repurchase && t2[3] < score.TotalScore {
		if t3[0] < score.TotalWater && t3[1] < score.TotalNumber && t3[2] < score.Repurchase && t3[3] < score.TotalScore {
			if t4[0] < score.TotalWater && t4[1] < score.TotalNumber && t4[2] < score.Repurchase && t4[3] < score.TotalScore {
				if t5[0] < score.TotalWater && t5[1] < score.TotalNumber && t5[2] < score.Repurchase && t5[3] < score.TotalScore {
					level = 5
				}
				level = 4
			}
			level = 3
		}
		level = 2
	}
	score.Discounts = level
	icelog.Info(score, "大神潜力等级分数")
	return score

}

// 计算分数
func (dao *Dao) CalculateScore(godId, gameId, days int64) model.StatisticsLevel {
	days = 0 - days
	end_time := time.Now().AddDate(0, 0, int(days)).Unix()
	// 获取总流水
	var TotalWater int
	var Water model.StatisticsLevel
	err := dao.dbr.Table("play_order").
		Select("sum(price) as  prices,sum(discount) as discounts").
		Where("god=? and game_id=? and state=? and create_time > ?", godId, gameId, 8, end_time).
		First(&Water).Error
	if err == nil {
		TotalWater = Water.Prices - Water.Discounts
	}

	// 复购率
	var OrderBuy []model.StatisticsLevel
	err = dao.dbr.Table("play_order").
		Select("count(*) as prices,buyer as discounts").
		Where("god=? and game_id=?  and state=? and create_time > ?", godId, gameId, 8, end_time).
		Group("buyer").
		Find(&OrderBuy).Error
	if err == nil {

	}
	// 接单人数
	number := len(OrderBuy)
	UserNum1, UserNum2, totalMoney := 0, 0, 0
	for i := 0; i < number; i++ {
		UserNum2++
		if OrderBuy[i].Prices > 1 {
			UserNum1++
		}
		totalMoney += OrderBuy[i].Prices
	}
	repurchase := (float32(UserNum1) / float32(UserNum2)) * 100

	// 历史评分
	var Score model.StatisticsLevel
	err = dao.dbr.Table("play_order_comment").
		Select("sum(score) as total_score").
		Where("god_id=? and create_time > ?", godId, end_time).
		First(&Score).Error

	return model.StatisticsLevel{
		TotalScore:  Score.TotalScore,
		Repurchase:  int(repurchase),
		TotalWater:  TotalWater,
		TotalNumber: number,
	}

}
