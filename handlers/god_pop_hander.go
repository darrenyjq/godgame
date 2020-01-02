package handlers

import (
	"fmt"
	"godgame/core"
	"iceberg/frame"
	"iceberg/frame/icelog"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	"github.com/nsqio/go-nsq"
	feedpb "laoyuegou.pb/feed/pb"
	imcourierpb "laoyuegou.pb/imcourier/pb"
)

type GodImOnline struct {
	dao *core.Dao
}

// 监听大神上下线事件 记录到ES中
func (self *GodImOnline) HandleMessage(msg *nsq.Message) error {
	var message imcourierpb.IMEventMsg
	err := proto.Unmarshal(msg.Body, &message)
	if err != nil {
		icelog.Error(err.Error())
		return err
	}

	godID := self.dao.GetGod(message.ClientInfo.ClientId).ID
	var isGod bool = false
	if godID > 0 {
		isGod = true
	}
	c := self.dao.Cpool.Get()
	defer c.Close()
	// icelog.Debug("记录上下线！：", message.Event, message.ClientInfo.ClientId)
	if message.Event == imcourierpb.IMEvent_IMEventOnline {
		if isGod {
			c.Do("sadd", core.RkOnlineGods(), godID)
		}
		self.esUpdate(message.ClientInfo.ClientId, fmt.Sprintf("%s", "lts"))
	}

	if message.Event == imcourierpb.IMEvent_IMEventOffline && message.ClientInfo.ClientId > 0 {
		if isGod {
			c.Do("srem", core.RkOnlineGods(), godID)
		}
		arr, _ := redis.Int64(c.Do("scard", core.RKGodAutoGrabGames(message.ClientInfo.ClientId)))
		// 已开启自动接单时 计入集合

		icelog.Info("抢单大神 离线事件：", message.Event, arr, core.RKGodAutoGrabGames(message.ClientInfo.ClientId))
		if arr > 0 {
			Rkey := core.RkGodOfflineTime()
			c.Do("zadd", Rkey, time.Now().Unix(), message.ClientInfo.ClientId)
		}
	}
	return nil
}

// 更新es大神池
func (self *GodImOnline) esUpdate(godId int64, lineTime string) {
	data := self.dao.EsQueryQuickOrder(godId)
	// 删除大神 离线时间
	self.dao.DelOffLineTime(godId)
	if len(data) > 0 {
		for _, item := range data {
			self.dao.EsUpdateQuickOrder(item.Id, map[string]interface{}{
				lineTime: time.Now(),
			})
			// 刷新ES 大神池
			Query := map[string]interface{}{
				"god_id": strconv.FormatInt(godId, 10),
			}
			// 获取该用户位置坐标
			resp, err := feedpb.UserPosition(frame.TODO(), &feedpb.UserPositionReq{UserId: godId})

			if err == nil && resp.GetData() != nil {

				resp.GetData().GetLon()
				Data := map[string]interface{}{
					"location2['lat']": float64(resp.GetData().GetLat()),
					"location2['lon']": float64(resp.GetData().GetLon()),
				}

				self.dao.ESUpdateGodGameByQuery(Query, Data)
			}

		}
	}
}
