package handlers

import (
	"github.com/nsqio/go-nsq"
	"godgame/core"
	"iceberg/frame/icelog"
	"laoyuegou.pb/imapi/pb"
)

type AutoGrabOrderHandler struct {
	dao *core.Dao
}

// 监听im消息 关闭自动抢单开关
func (self *AutoGrabOrderHandler) HandleMessage(msg *nsq.Message) error {
	var message imapipb.InternalContentMessage
	err := json.Unmarshal(msg.Body, &message)
	if err != nil {
		icelog.Error(err.Error())
		return nil
	}
	//检查是否为抢单大神

	// res, err := self.dao.GetGrabBedGodsOfBoss(message.R)
	// if err != nil || res == 1 {
	// 	return nil
	// } else if message.S == message.R[0] {
	// 	不给自己发邀请
	// return nil
	// }

	// list := dao.GlobalBaseDao.GameList()
	// resp := make(map[string]interface{})
	// resp["desc"] = "我们的小游戏"
	// resp["games"] = list
	// ms, _ := json.Marshal(resp)
	// imapipb.SendMessage(frame.TODO(), &imapipb.SendMessageReq{
	// 	Thread:      imapipb.CreateNotificationMessageThread(50001).ThreadString(),
	// 	FromId:      message.S,
	// 	ToId:        message.R[0],
	// 	ContentType: imapipb.MESSAGE_CONTENT_TYPE_NEW_CMD,
	// 	Subtype:     50001,
	// 	Message:     string(ms),
	// 	Pt:          imapipb.PLATFORM_TYPE_PLATFORM_TYPE_APP,
	// })
	//
	// resp["desc"] = "邀请你来约战"
	// imapipb.SendMessage(frame.TODO(), &imapipb.SendMessageReq{
	// 	Thread:      imapipb.CreateNotificationMessageThread(50001).ThreadString(),
	// 	FromId:      message.R[0],
	// 	ToId:        message.S,
	// 	ContentType: imapipb.MESSAGE_CONTENT_TYPE_NEW_CMD,
	// 	Subtype:     50001,
	// 	Message:     string(ms),
	// 	Pt:          imapipb.PLATFORM_TYPE_PLATFORM_TYPE_APP,
	// })
	// dao.GlobalBaseDao.CacheStore.SetFightTip(message.S, message.R[0])
	// icelog.Infof("成功发起邀约提示~  %d 发给 %d", message.R[0], message.S)
	return nil

}
