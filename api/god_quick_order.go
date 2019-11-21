package api

import (
	"context"
	"fmt"
	"github.com/olivere/elastic"
	"iceberg/frame"
	"iceberg/frame/icelog"
	"laoyuegou.com/util"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	"strings"
	"time"
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
				gg.ESAddQuickOrderInternal(params.ESQuickOrder)
			}
		case <-gg.exitChan:
			goto exit
		}
	}
exit:
	icelog.Info("exiting loop...")
}

func (gg *GodGame) BuildESQuickOrder(godID, gameID int64) (model.ESQuickOrder, error) {
	var result model.ESQuickOrder
	if godID == 0 || gameID == 0 {
		return result, fmt.Errorf("get god info error %d-%d", godID, gameID)
	}
	godInfo := gg.dao.GetGod(godID)
	if godInfo.UserID != godID {
		return result, fmt.Errorf("get god info error %d-%d", godID, gameID)
	}

	godGame := gg.dao.GetGodGame(godID, gameID)
	if godGame.UserID == 0 {
		return result, fmt.Errorf("god game not found %d-%d", godID, gameID)
	}

	accpetOrderSetting, err := gg.dao.GetGodSpecialAcceptOrderSetting(godID, gameID)
	if err != nil {
		return result, fmt.Errorf("price id error %d-%d %s", godID, gameID, err.Error())
	}
	Score := gg.dao.GetGodPotentialLevel(godID, gameID)
	result.GameID = gameID
	result.GodID = godID
	result.Gender = godInfo.Gender
	result.Price = accpetOrderSetting.PriceID
	result.UpdateTime = util.XTime(time.Now())
	result.LevelID = accpetOrderSetting.Levels
	result.RegionID = accpetOrderSetting.Regions
	result.PotentialLevel = Score.Discounts
	result.TotalScore = Score.TotalScore
	result.Repurchase = Score.Repurchase
	result.TotalWater = Score.TotalWater
	result.TotalNumber = Score.TotalNumber
	return result, nil
}

func (gg *GodGame) ESAddQuickOrderInternal(godGame model.ESQuickOrder) error {
	_, err := gg.esClient.Index().Index(gg.cfg.ES.PWQuickOrder).
		Type(gg.cfg.ES.PWType).
		Id(fmt.Sprintf("%d-%d", godGame.GodID, godGame.GameID)).
		BodyJson(godGame).
		Do(context.Background())
	if err != nil {
		icelog.Errorf("ESAddQuickOrder %+v error %s", godGame, err)
	}
	return nil

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
		icelog.Info("删除结果：", err, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (gg *GodGame) ESQueryQuickOrder(req godgamepb.QueryQuickOrderReq) []string {

	searchService := gg.esClient.Search().Index(gg.cfg.ES.PWQuickOrder).Type(gg.cfg.ES.PWType)
	query := elastic.NewBoolQuery()

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
		From(0).
		Size(100).
		Sort("update_time", false). // 倒序
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
			if seq := strings.Split(item.Id, "-"); len(seq) == 2 {
				res = append(res, seq[0])
			}
		}
	}
	return res

}

// 急速接单开关
func (gg *GodGame) AcceptQuickOrder(c frame.Context) error {
	var in godgamepb.AcceptQuickOrderReq
	if err := c.Bind(&in); err != nil || in.GodId == 0 || in.GameId == 0 || in.GrabSwitch == 0 {
		return c.RetBadRequestError("params fails")
	}
	if in.GrabSwitch == constants.GRAB_SWITCH5_OPEN {
		var data model.ESQuickOrder
		data, err := gg.BuildESQuickOrder(in.GodId, in.GameId)
		if err != nil {
			return c.RetBadRequestError(err.Error())
		}
		gg.ESAddQuickOrder(data)
	} else {
		esId := fmt.Sprintf("%d-%d", in.GodId, in.GameId)
		gg.ESDeleteQuickOrder([]string{esId})
	}
	return c.JSON2(StatusOK_V3, "success", nil)
}

// 急速接单匹配
func (gg *GodGame) QueryQuickOrder(c frame.Context) error {
	var in godgamepb.QueryQuickOrderReq
	if err := c.Bind(&in); err != nil {
		return c.RetBadRequestError("params fails")
	}
	data := gg.ESQueryQuickOrder(in)

	if data == nil {
		return c.RetBadRequestError("not find result")
	}
	return c.JSON2(StatusOK_V3, "success", data)
}
