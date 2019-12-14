package core

import (
	"context"
	"fmt"
	"gopkg.in/olivere/elastic.v5"
	"iceberg/frame/icelog"
	"reflect"
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

//  Dao层 更新大神池数据
func (dao *Dao) ESUpdateGodGameByQuery(query, data map[string]interface{}) error {
	var err error
	var valueType string
	builderRedefine := dao.EsClient.UpdateByQuery().Index(dao.Cfg.ES.PWIndexRedefine).Type(dao.Cfg.ES.PWType)

	for k, v := range data {
		valueType = reflect.TypeOf(v).String()
		if valueType == "int" || valueType == "int64" {
			builderRedefine = builderRedefine.Script(elastic.NewScriptInline(fmt.Sprintf("ctx._source.%s=%v", k, v)))

		} else {
			builderRedefine = builderRedefine.Script(elastic.NewScriptInline(fmt.Sprintf("ctx._source.%s='%v'", k, v)))
		}
	}
	for k, v := range query {
		builderRedefine = builderRedefine.Query(elastic.NewTermQuery(k, v))
	}
	_, err = builderRedefine.Do(context.Background())
	icelog.Info("更新ESRedefine部分字段结果 %+v, %+v error %s", query, data, err)
	return nil
}
