package handlers

import (
	"context"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/olivere/elastic"
	"godgame/core"
	"iceberg/frame/icelog"
	"iceberg/frame/protocol"
	"laoyuegou.com/http_api"
	"laoyuegou.com/util"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/imcourier/pb"
	"time"
)

type GodImOnline struct {
	dao *core.Dao
}

// 监听大神上下线事件 记录到ES中
func (self *GodImOnline) HandleMessage(msg *nsq.Message) error {
	var message imcourierpb.IMEventMsg
	err := proto.Unmarshal(msg.Body, &message)
	// icelog.Infof("########  %v", message)
	if err != nil {
		icelog.Error(err.Error())
		return err
	}
	if message.Event == imcourierpb.IMEvent_IMEventOnline {
		self.esUpdate(message.ClientInfo.ClientId, fmt.Sprintf("%s", "lts"))

	}

	if message.Event == imcourierpb.IMEvent_IMEventOffline {
		// icelog.Info("离线了！！！！！", message)
		// self.esQueryQuickOrder(message.ClientInfo.ClientId, fmt.Sprintf("%s", "offlinetime"))
		go self.OffLineTimer(message.ClientInfo.ClientId)
	}
	return nil
}

// 离线1小时候自动 关闭抢单
func (self *GodImOnline) OffLineTimer(userId int64) {
	ticker := time.NewTimer(3600 * time.Second)
	defer ticker.Stop()
	select {
	case <-ticker.C:
		icelog.Info("用户离线，关闭自动接单", userId)
		list := self.esQueryQuickOrder(userId)

		for _, item := range list {
			var GodInfo model.ESQuickOrder
			if err := json.Unmarshal(*item.Source, &GodInfo); err != nil {
				continue
			}
			icelog.Info(GodInfo.OnlineTime)
			formatTime, err := time.Parse("2006-01-02 15:04:05", util.XTime.String(GodInfo.OnlineTime))
			if err == nil {
				diff := time.Now().Unix() - formatTime.Unix()
				if diff > 3600 {
					// 大于1小时 自动下线
					self.PhpHttps(userId)
				}
			}

		}
	}
}

func (self *GodImOnline) PhpHttps(godId int64) {
	client := http_api.NewClient()
	url := fmt.Sprintf("%s%s", self.dao.Cfg.Urls["php_api"], "order/interior/quickorder/disable-auto-grab")
	resp, err := client.POSTV2(url, map[string]interface{}{
		"god_id": godId,
	})
	if err != nil {
		icelog.Error(err.Error())
	}
	if resp.StatusCode == 200 {
		qq, _ := resp.ReadAll()
		var ress protocol.Message
		err = json.Unmarshal(qq, &ress)
		icelog.Info("离线超时 关闭自动抢单功能", ress.Errmsg)
	}
}

// 查询大神池 更新es
func (self *GodImOnline) esUpdate(godId int64, lineTime string) {
	data := self.esQueryQuickOrder(godId)
	if len(data) > 0 {
		for _, item := range data {
			self.esUpdateQuickOrder(item.Id, map[string]interface{}{
				lineTime: util.XTime(time.Now()),
			})
		}
	}
}

// 更新ES 数据
func (self *GodImOnline) esUpdateQuickOrder(id string, data map[string]interface{}) {
	_, err := self.dao.EsClient.Update().
		Index(self.dao.Cfg.ES.PWQuickOrder).
		Type(self.dao.Cfg.ES.PWType).
		Id(id).
		Doc(data).
		Do(context.Background())
	if err != nil {
		icelog.Info("急速接单大神池更新失败：", id, err.Error())
	}
}

// 查询es数据
func (self *GodImOnline) esQueryQuickOrder(godId int64) []*elastic.SearchHit {
	searchService := self.dao.EsClient.Search().Index(self.dao.Cfg.ES.PWQuickOrder).Type(self.dao.Cfg.ES.PWType)
	query := elastic.NewBoolQuery().Should(elastic.NewTermQuery("god_id", godId))
	resp, err := searchService.Query(query).
		From(0).
		Size(20).
		// Sort("update_time", false). // 倒序
		Pretty(true).
		Do(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Printf("query cost %d millisecond.\n", resp.TookInMillis)

	if err != nil || resp.Hits.TotalHits == 0 {
		return nil
	}
	if resp != nil {
		return resp.Hits.Hits
	}
	return nil
}
