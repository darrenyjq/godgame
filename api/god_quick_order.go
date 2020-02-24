package api

import (
	"context"
	"fmt"
	"gopkg.in/olivere/elastic.v5"
	"iceberg/frame"
	"iceberg/frame/icelog"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	"strings"
)

const (
	ES_ORDER_DELETE       = byte(1)
	ES_ORDER_BATCH_DELETE = byte(2)
	ES_ORDER_ADD          = byte(3)
	ES_ORDER_UPDATE       = byte(4)
	ES_ORDER_BY_QUERY     = byte(5)
)

type ESOrderParams struct {
	Action       byte
	IDs          []string
	Query        map[string]interface{}
	Data         map[string]interface{}
	ESQuickOrder model.ESQuickOrder
}

func (gg *GodGame) ESAddQuickOrder(godGame model.ESQuickOrder) {
	params := ESOrderParams{
		Action:       ES_ACTION_ADD,
		ESQuickOrder: godGame,
	}
	gg.esQuickOrderChan <- params
}

func (gg *GodGame) StartQuickOrderLoop() {
	for {
		select {
		case params, ok := <-gg.esQuickOrderChan:
			if !ok {
				goto exit
			}
			switch params.Action {
			case ES_ORDER_DELETE, ES_ORDER_BATCH_DELETE:
				gg.ESDeleteQuickOrder(params.IDs)
			case ES_ORDER_UPDATE:
				gg.ESUpdateQuickOrder(params.IDs[0], params.Data)
			case ES_ORDER_ADD:
				gg.dao.ESAddQuickOrderInternal(params.ESQuickOrder)
			}
		case <-gg.exitChan:
			goto exit
		}
	}
exit:
	icelog.Info("exiting loop...")
}

func (gg *GodGame) ESUpdateQuickOrder(id string, data map[string]interface{}) {
	_, err := gg.esClient.Update().
		Index(gg.cfg.ES.PWQuickOrder).
		Type(gg.cfg.ES.PWType).
		Id(id).
		Doc(data).
		Do(context.Background())
	if err != nil {
		icelog.Info("急速接单大神池更新失败：", id, err.Error())
	}
}

// 删除 急速接单池
func (gg *GodGame) ESDeleteQuickOrder(esIDs []string) error {
	for _, id := range esIDs {
		_, err := gg.esClient.Delete().Index(gg.cfg.ES.PWQuickOrder).Type(gg.cfg.ES.PWType).
			Id(id).
			Do(context.Background())

		icelog.Info("急速接单池 删除结果：", err, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (gg *GodGame) ESQueryQuickOrder(req godgamepb.QueryQuickOrderReq) []*elastic.SearchHit {

	searchService := gg.esClient.Search().Index(gg.cfg.ES.PWQuickOrder).Type(gg.cfg.ES.PWType)
	query := elastic.NewBoolQuery()

	if req.GetGodId() > 0 {
		query = query.Must(elastic.NewTermQuery("god_id", req.GodId))
	}

	if req.GetGameId() > 0 {
		query = query.Must(elastic.NewTermQuery("game_id", req.GameId))
	}

	if req.GetGender() > 0 {
		query = query.Should(elastic.NewTermQuery("gender", req.Gender))
	}

	if req.GetLevelId() > 0 {
		query = query.Should(elastic.NewTermQuery("level_id", req.LevelId))
	}

	if req.GetPriceId() > 0 {
		query = query.Should(elastic.NewTermQuery("price_id", req.PriceId))
	}

	if req.GetRegionId() > 0 {
		query = query.Should(elastic.NewTermQuery("region_id", req.RegionId))
	}

	resp, err := searchService.Query(query).
		// From(0).
		Size(300).
		Sort("update_time", false). // 倒序
		// Pretty(true).
		Do(context.Background())

	if err != nil {
		icelog.Debug(err.Error())
		return nil
	}
	// fmt.Printf("query cost %d millisecond.\n", resp.TookInMillis)
	if resp.Hits.TotalHits == 0 {
		return nil
	}
	if resp != nil {
		return resp.Hits.Hits

	}
	return nil

}

func (gg *GodGame) GetQuickOrderIds(resp []*elastic.SearchHit) []string {
	res := []string{}
	for _, item := range resp {
		if seq := strings.Split(item.Id, "-"); len(seq) == 2 {
			res = append(res, seq[0])
		}
	}
	return res
}

// 刷新急速接单池
func (gg *GodGame) FlashAllQuickOrder(c frame.Context) error {
	var req godgamepb.FlashAllQuickOrderReq
	if err := c.Bind(&req); err == nil {
		// 刷新单个大神 及品类
		if req.GetGodId() > 0 {
			go func() {
				lists, err := gg.dao.GetAcceptSettings(req.GetGodId())
				if err == nil && len(lists) > 0 {
					for _, v := range lists {
						var data model.ESQuickOrder
						data, err := gg.dao.BuildESQuickOrder(v.GodID, v.GameID)
						if err != nil {
							continue
						}
						gg.ESAddQuickOrder(data)
					}
				}
			}()
			return c.RetSuccess("success 已经异步刷新大神池，请不要频繁操作", nil)
		}
		// 刷新全部大神 及品类  标识game==100
		if req.GetGameId() == 100 {
			go func() {
				lists, err := gg.dao.GetQuickOrderGods()
				if err == nil && len(lists) > 0 {
					for _, v := range lists {
						var data model.ESQuickOrder
						data, err := gg.dao.BuildESQuickOrder(v.GodID, v.GameID)
						if err != nil {
							return
						}
						gg.ESAddQuickOrder(data)
					}
					return
				}

			}()
			return c.RetSuccess("success 已经异步刷新大神池，请不要频繁操作", nil)
		}

	}

	return c.RetSuccess("没有大神开启急速接单", nil)
}

// 急速接单开关
func (gg *GodGame) AcceptQuickOrder(c frame.Context) error {
	var in godgamepb.AcceptQuickOrderReq
	if err := c.Bind(&in); err != nil || in.GodId == 0 || in.GameId == 0 || in.GrabSwitch == 0 {
		return c.RetBadRequestError("params fails")
	}
	if in.GrabSwitch == constants.GRAB_SWITCH5_OPEN {
		gg.dao.AcceptQuickOrderSetting(in.GodId, in.GameId, constants.GRAB_SWITCH5_OPEN)
		var data model.ESQuickOrder
		data, err := gg.dao.BuildESQuickOrder(in.GodId, in.GameId)
		if err != nil {
			return c.RetBadRequestError(err.Error())
		}
		gg.ESAddQuickOrder(data)
	} else {
		esId := fmt.Sprintf("%d-%d", in.GodId, in.GameId)
		gg.ESDeleteQuickOrder([]string{esId})
		gg.dao.AcceptQuickOrderSetting(in.GodId, in.GameId, constants.GRAB_SWITCH5_CLOSE)
		gg.dao.CloseAutoGrabOrder(in.GodId, in.GameId)
	}
	// 删除大神数据缓存
	gg.dao.DelGodInfoCache(in.GodId, in.GameId)
	return c.JSON2(StatusOK_V3, "success", nil)
}

// 获取急速接单池数据
func (gg *GodGame) QueryQuickOrder(c frame.Context) error {
	var in godgamepb.QueryQuickOrderReq
	if err := c.Bind(&in); err != nil {
		return c.RetBadRequestError("params fails")
	}
	if data := gg.ESQueryQuickOrder(in); data != nil && len(data) > 0 {
		var into godgamepb.QueryQuickOrderResp_Data
		json.Unmarshal(*data[0].Source, &into)
		return c.JSON2(StatusOK_V3, "success", into)
	}
	return c.RetBadRequestError("not find result")
}

// 重载 监控 chan
func (gg *GodGame) ReloadGodGameLoop(c frame.Context) error {
	var in godgamepb.ReloadGodGameLoopReq
	if err := c.Bind(&in); err != nil {
		return c.RetBadRequestError("params fails")
	}
	gg.dao.ReloadChanLoop(in.Signal)
	return c.RetSuccess("success", nil)
}
