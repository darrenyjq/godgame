package handlers

import (
	"context"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/nsqio/go-nsq"
	"github.com/olivere/elastic"
	"godgame/core"
	"iceberg/frame/icelog"
	"laoyuegou.com/util"
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
	icelog.Infof("########  %v", message)
	if err != nil {
		icelog.Error(err.Error())
		return err
	}
	if message.Event == imcourierpb.IMEvent_IMEventOnline {
		self.esQueryQuickOrder(message.ClientInfo.ClientId, fmt.Sprintf("%s", "lts"))

	}

	if message.Event == imcourierpb.IMEvent_IMEventOffline {
		self.esQueryQuickOrder(message.ClientInfo.ClientId, fmt.Sprintf("%s", "offlinetime"))
	}
	return nil
}

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

func (self *GodImOnline) esQueryQuickOrder(godId int64, lineTime string) []string {
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

	if err != nil {
		return nil
	}
	if resp.Hits.TotalHits == 0 {
		return nil
	}
	res := []string{}
	if resp != nil {
		for _, item := range resp.Hits.Hits {
			self.esUpdateQuickOrder(item.Id, map[string]interface{}{
				lineTime: util.XTime(time.Now()),
			})
		}
	}
	return res

}
