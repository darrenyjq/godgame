package core

import (
	"context"
	"fmt"
	"github.com/olivere/elastic"
	"iceberg/frame/icelog"
	"iceberg/frame/protocol"
	"laoyuegou.com/http_api"
)

// 查询es数据
func (dao *Dao) EsQueryQuickOrder(godId int64) []*elastic.SearchHit {
	searchService := dao.EsClient.Search().Index(dao.Cfg.ES.PWQuickOrder).Type(dao.Cfg.ES.PWType)
	query := elastic.NewBoolQuery().Should(elastic.NewTermQuery("god_id", godId))
	resp, err := searchService.Query(query).
		From(0).
		Size(20).
		// Sort("update_time", false). // 倒序
		Pretty(true).
		Do(context.Background())
	if err != nil {
		icelog.Debug(err.Error())
		return nil
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

// 更新ES 数据
func (dao *Dao) EsUpdateQuickOrder(id string, data map[string]interface{}) {
	_, err := dao.EsClient.Update().
		Index(dao.Cfg.ES.PWQuickOrder).
		Type(dao.Cfg.ES.PWType).
		Id(id).
		Doc(data).
		Do(context.Background())
	if err != nil {
		icelog.Info("急速接单大神池更新失败：", id, err.Error())
	}
}
func (dao *Dao) PhpHttps(godId, reason int64) {
	client := http_api.NewClient()
	url := fmt.Sprintf("%s%s", dao.Cfg.Urls["php_api"], "order/interior/quickorder/disable-auto-grab")
	resp, err := client.POSTV2(url, map[string]interface{}{
		"god_id": godId,
		"reason": reason,
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
