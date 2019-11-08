package api

import (
	"context"
	"fmt"
	"github.com/olivere/elastic"
	"godgame/core"
	"iceberg/frame/icelog"
	"laoyuegou.com/util"
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
			// case ES_ORDER_BY_QUERY:
			// 	gg.ESQueryQuickOrder(params.IDs[0])
			case ES_ORDER_UPDATE:
				gg.ESUpdateQuickOrder(params.IDs)
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
	result.GameID = gameID
	result.GodID = godID
	result.Gender = godInfo.Gender
	result.Price = accpetOrderSetting.PriceID
	result.UpdateTime = util.XTime(time.Now())
	result.LevelID = accpetOrderSetting.Levels
	result.RegionID = accpetOrderSetting.Regions
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

func (gg *GodGame) ESUpdateQuickOrder(esIDs []string) error {
	return nil

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

func (gg *GodGame) ESQueryQuickOrder(req godgamepb.QueryQuickOrderReq) error {
	searchService := gg.esClient.Scroll(gg.cfg.ES.PWQuickOrder)
	query := elastic.NewBoolQuery().
		Must(elastic.NewTermQuery("game_id", req.GameId)).
		Should(elastic.NewTermQuery("gender", req.Gender)).
		Should(elastic.NewTermQuery("level_id", req.LevelId)).
		Should(elastic.NewTermQuery("price_id", req.PriceId)).
		Should(elastic.NewTermQuery("region_id", req.RegionId))

	data, err := searchService.Query(query).
		Sort("update_time", false). // 倒序
		Do(context.Background())

	if err != nil {
		if err.Error() == "EOF" {
			return nil
		}
	}
	redisConn := gg.dao.GetRedisPool().Get()
	defer redisConn.Close()
	key := core.RKQuickOrder(1, 1)

	for _, item := range data.Hits.Hits {
		if seq := strings.Split(item.Id, "-"); len(seq) == 2 {
			redisConn.Do("SADD", key, seq[0])
		}
	}
	return nil

}
