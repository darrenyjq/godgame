package api

// 管理Elasticsearch中的大神信息
import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olivere/elastic"
	"iceberg/frame"
	"iceberg/frame/icelog"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	order_const "laoyuegou.pb/plorder/constants"
	"laoyuegou.pb/plorder/pb"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	ES_ACTION_DELETE          = byte(1)
	ES_ACTION_BATCH_DELETE    = byte(2)
	ES_ACTION_ADD             = byte(3)
	ES_ACTION_UPDATE          = byte(4)
	ES_ACTION_UPDATE_BY_QUERY = byte(5)
)

type ESParams struct {
	Action    byte
	IDs       []string
	Query     map[string]interface{}
	Data      map[string]interface{}
	ESGodGame model.ESGodGame
}

func (gg *GodGame) StartLoop() {
	for {
		select {
		case params, ok := <-gg.esChan:
			if !ok {
				icelog.Errorf("%s", ok)
			}

			switch params.Action {
			case ES_ACTION_UPDATE:
				gg.ESUpdateGodGame(params.IDs[0], params.Data)
			case ES_ACTION_UPDATE_BY_QUERY:
				gg.ESUpdateGodGameByQuery(params.Query, params.Data)
			case ES_ACTION_DELETE:
				gg.ESDeleteGodGame(params.IDs[0])
			case ES_ACTION_BATCH_DELETE:
				gg.ESBatchDeleteByID(params.IDs)
			case ES_ACTION_ADD:
				gg.ESAddGodGameInternal(params.ESGodGame)
			}
		}
	}
}

// 修改Elasticsearch里面的大神陪玩信息
// 可以只更新指定列
func (gg *GodGame) RefreshESGodGame(c frame.Context) error {
	var req godgamepb.RefreshESGodGameReq
	var err error
	if err = c.Bind(&req); err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "", nil)
	} else if req.GetEsId() == "" && req.GetQuery() == nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "invalid condition", nil)
	} else if req.GetData() == nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, "invalid data", nil)
	}
	data := make(map[string]interface{})
	var tmpIntger int64
	for k, v := range req.GetData() {
		// 强制将LTS刷新为当前时间
		if k == "lts" {
			data[k] = time.Now()
		} else {
			if tmpIntger, err = strconv.ParseInt(v, 10, 64); err == nil {
				data[k] = tmpIntger
			} else {
				data[k] = v
			}
		}
	}
	if req.GetEsId() != "" {
		// err = gg.ESUpdateGodGame(req.GetEsId(), data)
		params := ESParams{
			Action: ES_ACTION_UPDATE,
			IDs:    []string{req.GetEsId()},
			Data:   data,
		}
		gg.esChan <- params
	} else {
		query := make(map[string]interface{})
		for k, v := range req.GetQuery() {
			query[k] = v
		}
		// err = gg.ESUpdateGodGameByQuery(query, data)
		params := ESParams{
			Action: ES_ACTION_UPDATE_BY_QUERY,
			Query:  query,
			Data:   data,
		}
		gg.esChan <- params
	}
	return c.JSON2(StatusOK_V3, "", nil)
}

func (gg *GodGame) ESGetGodGame(id string) (model.ESGodGame, error) {
	var result model.ESGodGame
	ret, err := gg.esClient.Get().Index(gg.cfg.ES.PWIndex).Type(gg.cfg.ES.PWType).
		Id(id).Do(context.Background())
	if err != nil {
		if strings.Index(err.Error(), "Error 404") == -1 {
			icelog.Errorf("ESGetGodGame id:%s error %s", id, err)
		}
		return result, err
	}
	if ret.Found {
		err = json.Unmarshal(*ret.Source, &result)
	}
	return result, err
}

func (gg *GodGame) ESAddGodGame(godGame model.ESGodGame) error {
	params := ESParams{
		Action:    ES_ACTION_ADD,
		ESGodGame: godGame,
	}
	gg.esChan <- params
	return nil
}

func (gg *GodGame) ESAddGodGameInternal(godGame model.ESGodGame) error {
	_, err := gg.esClient.Index().Index(gg.cfg.ES.PWIndex).Type(gg.cfg.ES.PWType).
		Id(fmt.Sprintf("%d-%d", godGame.GodID, godGame.GameID)).
		BodyJson(godGame).
		Do(context.Background())
	if err != nil {
		icelog.Errorf("ESAddGodGameInternal %+v error %s", godGame, err)
		return err
	}
	return nil
}

func (gg *GodGame) ESUpdateGodGameByQuery(query, data map[string]interface{}) error {
	var err error
	builder := gg.esClient.UpdateByQuery().Index(gg.cfg.ES.PWIndex).Type(gg.cfg.ES.PWType)
	var valueType string
	for k, v := range data {
		valueType = reflect.TypeOf(v).String()
		if valueType == "int" || valueType == "int64" {
			builder = builder.Script(elastic.NewScriptInline(fmt.Sprintf("ctx._source.%s=%v", k, v)))
		} else {
			builder = builder.Script(elastic.NewScriptInline(fmt.Sprintf("ctx._source.%s='%v'", k, v)))
		}
	}
	for k, v := range query {
		builder = builder.Query(elastic.NewTermQuery(k, v))
	}
	_, err = builder.Do(context.Background())
	if err != nil {
		icelog.Errorf("ESUpdateGodGameByQuery %+v, %+v error %s", query, data, err)
		return err
	}
	return nil
}

func (gg *GodGame) ESBatchDeleteByID(esIDs []string) error {
	if len(esIDs) == 0 {
		return nil
	}
	bulkRequest := gg.esClient.Bulk()
	for _, esID := range esIDs {
		bulkRequest.Add(elastic.NewBulkDeleteRequest().Index(gg.cfg.ES.PWIndex).
			Type(gg.cfg.ES.PWType).Id(esID))
	}
	if bulkRequest.NumberOfActions() != len(esIDs) {
		return fmt.Errorf("NumberOfActions[%d] != esIDs[%d]", bulkRequest.NumberOfActions(), len(esIDs))
	}
	_, err := bulkRequest.Do(context.Background())
	return err
}

func (gg *GodGame) ESUpdateGodGame(id string, data map[string]interface{}) error {
	_, err := gg.esClient.Update().Index(gg.cfg.ES.PWIndex).Type(gg.cfg.ES.PWType).
		Id(id).Doc(data).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (gg *GodGame) ESDeleteGodGame(id string) error {
	_, err := gg.esClient.Delete().Index(gg.cfg.ES.PWIndex).Type(gg.cfg.ES.PWType).
		Id(id).
		Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}

// 组装Elasticsearch一行陪玩数据
func (gg *GodGame) BuildESGodGameData(godID, gameID int64) (model.ESGodGame, error) {
	var result model.ESGodGame
	godInfo := gg.dao.GetGod(godID)
	if godInfo.UserID != godID {
		return result, fmt.Errorf("get god info error %d-%d", godID, gameID)
	}
	godGame := gg.dao.GetGodGame(godID, gameID)
	if godGame.UserID == 0 {
		return result, fmt.Errorf("god game not found %d-%d", godID, gameID)
	} else if godGame.Recommend != constants.RECOMMEND_YES {
		return result, fmt.Errorf("not recommend %d-%d", godID, gameID)
	}
	accpetOrderSetting, err := gg.dao.GetGodSpecialAcceptOrderSetting(godID, gameID)
	if err != nil {
		return result, fmt.Errorf("price id error %d-%d %s", godID, gameID, err.Error())
	}
	sortResp, err := plorderpb.SortFactor(frame.TODO(), &plorderpb.SortFactorReq{
		GodId:  godID,
		GameId: gameID,
	})
	if err != nil || sortResp.GetErrcode() != 0 {
		return result, err
	}
	result.GodID = godID
	result.GameID = gameID
	result.Gender = godInfo.Gender
	result.PeiWanStatus = sortResp.GetData().GetStatus()
	result.OrderCnt = sortResp.GetData().GetGameCnt()
	result.SevenDaysCnt = sortResp.GetData().GetGameSeventCnt()
	result.SevenDaysHours = sortResp.GetData().GetGameSeventHoursCnt()
	result.RejectOrder = sortResp.GetData().GetBadOrder()
	result.Weight = gg.dao.GetGodGameWeight(godID, gameID)
	if sortResp.GetData().GetStatus() == order_const.PW_STATUS_BUSY {
		result.Weight = 0
	}
	result.PassedTime = godGame.Passedtime
	if godGame.PeiwanPriceType == order_const.PW_PRICE_TYPE_BY_OM {
		result.Price = godGame.PeiwanPrice
	} else {
		result.PriceID = accpetOrderSetting.PriceID
	}
	result.HighestLevelID = godGame.HighestLevelID
	return result, nil
}
