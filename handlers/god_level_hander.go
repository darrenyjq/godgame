package handlers

import (
	"github.com/nsqio/go-nsq"
	"godgame/core"
	"iceberg/frame/icelog"
	constants2 "laoyuegou.pb/plcomment/constants"
	"laoyuegou.pb/plorder/constants"
	"laoyuegou.pb/virgo/model"
)

type GodLevelHandler struct {
	dao *core.Dao
}

// 订单完成、新的评价触发重新计算大神等级
func (self *GodLevelHandler) HandleMessage(msg *nsq.Message) error {
	icelog.Debugf("GodLevelHandler received: %s", msg.Body)
	var message model.Record
	var err error
	err = json.Unmarshal(msg.Body, &message)
	if err != nil {
		icelog.Errorf("%s", err.Error())
		return nil
	}
	// icelog.Infof("%+v,,,,%+v,,,,,,,,,", message, message.Action)
	var godID, gameID int64
	// 审核评论状态通过变成1
	if message.Schema == "app" && message.Name == "play_order_comment" && message.Action == "update" {
		if int64(message.Columns[1]["state"].(float64)) != constants2.ORDER_COMMENT_STATE_OK {
			return nil
		}
		// 陪玩评价
		godID = int64(message.Columns[0]["god_id"].(float64))
		gameID = int64(message.Columns[0]["game_id"].(float64))

	} else if message.Schema == "app" && message.Name == "play_order_comment" && message.Action == "insert" {
		if int64(message.Columns[0]["state"].(float64)) != constants2.ORDER_COMMENT_STATE_OK {
			return nil
		}
		// 首次5星好评默认通过
		// 陪玩评价
		godID = int64(message.Columns[0]["god_id"].(float64))
		gameID = int64(message.Columns[0]["game_id"].(float64))

	} else if message.Schema == "app" && message.Name == "play_order" && message.Action == "update" {
		// 陪玩订单
		state := int64(message.Columns[1]["state"].(float64))
		if state != constants.ORDER_COMPLETED {
			return nil
		}
		godID = int64(message.Columns[1]["god"].(float64))
		gameID = int64(message.Columns[1]["game_id"].(float64))
	}
	if godID == 0 || gameID == 0 {
		return nil
	}
	icelog.Infof("【重算大神等级】大神：%d，品类：%d，事件类型：%s", godID, gameID, message.Name)
	err = self.dao.ReCalcGodLevel(godID, gameID)
	if err != nil {
		icelog.Errorf("GodLevelHandler error %s", err.Error())
		return err
	}

	// 刷新急速接单池
	self.dao.FlashGodQuickOrder(godID)
	return nil
}
