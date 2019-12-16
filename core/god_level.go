package core

import (
	"iceberg/frame"
	"iceberg/frame/icelog"
	"laoyuegou.pb/plcomment/pb"
	"laoyuegou.pb/plorder/pb"
)

// 重新计算大神陪玩等级
func (dao *Dao) ReCalcGodLevel(godID, gameID int64) error {
	if dao.Cfg.GodLevelIgnoreGames != nil {
		for _, v := range dao.Cfg.GodLevelIgnoreGames {
			if v == gameID {
				return nil
			}
		}
	}
	v1, err := dao.GetGodSpecialGameV1(godID, gameID)
	if err != nil {
		return err
	}
	commentResp, err := plcommentpb.GetGodGameComment(frame.TODO(), &plcommentpb.GetGodGameCommentReq{
		GodId:  godID,
		GameId: gameID,
	})
	if err != nil || commentResp == nil || commentResp.GetData() == nil {
		icelog.Errorf("GetGodGameComment[%d/%d] error:%s", godID, gameID, err)
		return err
	}
	v1.Score = commentResp.GetData().GetScore()

	orderResp, err := plorderpb.Count(frame.TODO(), &plorderpb.CountReq{
		GodId:  godID,
		GameId: gameID,
	})
	if err != nil || orderResp == nil || orderResp.GetData() == nil {
		icelog.Errorf("Get orderCount[%d] error:%s", godID, err)
		return err
	}
	v1.AcceptNum = orderResp.GetData().GetCompletedHoursAmount()
	totalCommentCnt := commentResp.GetData().GetCommentCnt()
	badCommentCnt := commentResp.GetData().GetBadCommentCnt()
	goodCommentRate := float64(1)
	if totalCommentCnt > 0 {
		goodCommentRate = float64(totalCommentCnt-badCommentCnt) / float64(totalCommentCnt)
	}
	level := v1.Level
	if v1.Level == 5 {
		if goodCommentRate < 0.98 {
			level = 4
		}
	} else {
		totalOrderCnt := orderResp.GetData().GetCompletedHoursAmount()
		if totalOrderCnt >= 50 && goodCommentRate >= 0.98 {
			level = 4
		} else if totalOrderCnt >= 20 && goodCommentRate >= 0.95 {
			level = 3
		} else if totalOrderCnt >= 3 && goodCommentRate >= 0.9 {
			level = 2
		} else {
			level = 1
		}
	}
	if level != v1.Level {
		v1.Level = level
		err = dao.dbw.Table("play_god_games").Where("userid=? AND gameid=?", godID, gameID).Update("god_level", level).Error
		if err != nil {
			icelog.Errorf("ReCalcGodLevel %d-%d error %s", godID, gameID, err)
		}
		// 根据大神等级 获取接单价格id 修改接单设置
		if err = dao.UpdateAcceptOrderInfo(level, gameID, godID); err != nil {
			return err
		}
	}
	c := dao.Cpool.Get()
	defer c.Close()
	c.Do("DEL", RKGodGameInfo(godID, gameID), RKOneGodGameV1(godID, gameID), RKSimpleGodGamesKey(godID))
	return nil
}
