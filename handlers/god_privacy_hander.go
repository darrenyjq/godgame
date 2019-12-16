package handlers

import (
	"github.com/nsqio/go-nsq"
	"godgame/core"
	"iceberg/frame/icelog"
	"laoyuegou.pb/virgo/model"
	"strconv"
)

type GodPrivacyHandler struct {
	dao *core.Dao
}

// 更新个人隐私 触发重新 跟新大神池信息
func (self *GodPrivacyHandler) HandleMessage(msg *nsq.Message) error {
	var message model.Record
	var err error
	err = json.Unmarshal(msg.Body, &message)
	if err != nil {
		icelog.Errorf("%s", err.Error())
		return nil
	}
	// icelog.Infof("GodPrivacyHandler received: %s", msg.Body)
	if message.Schema == "app" && message.Name == "privacy_cfg" && message.Action == "update" && message.Columns[0]["is_show_near"] != message.Columns[1]["is_show_near"] {

		// is_show_near 是否在展示在附近 1 展示 2 不展示
		IsShowNear := byte(message.Columns[1]["is_show_near"].(float64))
		godID := int64(message.Columns[1]["user_id"].(float64))

		icelog.Infof("【更新隐私】大神：%d，开关：%d", godID, IsShowNear)
		// 刷新ES 大神池
		Query := map[string]interface{}{
			"god_id": strconv.FormatInt(godID, 10),
		}
		Data := map[string]interface{}{
			"is_show_near": IsShowNear,
		}
		self.dao.ESUpdateGodGameByQuery(Query, Data)
	}
	return nil
}
