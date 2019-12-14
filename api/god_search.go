package api

// 管理Elasticsearch中的大神信息
import (
	"context"
	"fmt"
	"gopkg.in/olivere/elastic.v5"
	"iceberg/frame"
	"iceberg/frame/icelog"
	"laoyuegou.pb/godgame/constants"
	"laoyuegou.pb/godgame/model"
	"laoyuegou.pb/godgame/pb"
	order_const "laoyuegou.pb/plorder/constants"
	"laoyuegou.pb/plorder/pb"
	"laoyuegou.pb/user/pb"
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
	Action            byte
	IDs               []string
	Query             map[string]interface{}
	Data              map[string]interface{}
	ESGodGame         model.ESGodGame
	ESGodGameRedefine model.ESGodGameRedefine
}

func (gg *GodGame) StartLoop() {
	for {
		select {
		case params, ok := <-gg.esChan:
			if !ok {
				goto exit
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
				gg.ESAddGodGameInternal(params.ESGodGameRedefine)
			}
		case <-gg.exitChan:
			goto exit
		}
	}
exit:
	icelog.Info("exiting loop...")
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

func (gg *GodGame) ESGetGodGame(id string) (model.ESGodGameRedefine, error) {
	var result model.ESGodGameRedefine
	ret, err := gg.esClient.Get().Index(gg.cfg.ES.PWIndexRedefine).Type(gg.cfg.ES.PWType).
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

func (gg *GodGame) ESAddGodGame(godGame model.ESGodGameRedefine) error {
	params := ESParams{
		Action:            ES_ACTION_ADD,
		ESGodGameRedefine: godGame,
	}
	gg.esChan <- params
	return nil
}

func (gg *GodGame) ESAddGodGameInternal(godGame model.ESGodGameRedefine) error {
	_, err := gg.esClient.Index().Index(gg.cfg.ES.PWIndex).Type(gg.cfg.ES.PWType).
		Id(fmt.Sprintf("%d-%d", godGame.GodID, godGame.GameID)).
		BodyJson(godGame).
		Do(context.Background())

	godGameRedefine, res := gg.BuildESGodGameDataRedefine(godGame.GodID, godGame.GameID)
	if res == nil {
		_, result := gg.esClient.Index().Index(gg.cfg.ES.PWIndexRedefine).Type(gg.cfg.ES.PWType).
			Id(fmt.Sprintf("%d-%d", godGame.GodID, godGame.GameID)).
			BodyJson(godGameRedefine).
			Do(context.Background())
		if result != nil {
			icelog.Info("添加ESRedefine数据失败 %+v error %s", godGame, result)
		}
	} else {
		icelog.Info("添加ESRedefine数据失败", godGame, godGameRedefine, res)
	}

	if err != nil {
		icelog.Errorf("ESAddGodGameInternal %+v error %s", godGame, err)
		return err
	}
	return nil
}

func (gg *GodGame) ESUpdateGodGameByQuery(query, data map[string]interface{}) error {
	var err error
	var valueType string
	builder := gg.esClient.UpdateByQuery().Index(gg.cfg.ES.PWIndex).Type(gg.cfg.ES.PWType)
	builderRedefine := gg.esClient.UpdateByQuery().Index(gg.cfg.ES.PWIndexRedefine).Type(gg.cfg.ES.PWType)

	for k, v := range data {
		valueType = reflect.TypeOf(v).String()
		if valueType == "int" || valueType == "int64" {
			builder = builder.Script(elastic.NewScriptInline(fmt.Sprintf("ctx._source.%s=%v", k, v)))
			builderRedefine = builderRedefine.Script(elastic.NewScriptInline(fmt.Sprintf("ctx._source.%s=%v", k, v)))

		} else {
			builder = builder.Script(elastic.NewScriptInline(fmt.Sprintf("ctx._source.%s='%v'", k, v)))
			builderRedefine = builderRedefine.Script(elastic.NewScriptInline(fmt.Sprintf("ctx._source.%s='%v'", k, v)))
		}
	}
	for k, v := range query {
		builder = builder.Query(elastic.NewTermQuery(k, v))
		builderRedefine = builderRedefine.Query(elastic.NewTermQuery(k, v))
	}

	_, err = builderRedefine.Do(context.Background())
	icelog.Info("更新ESRedefine部分字段结果 %+v, %+v error %s", query, data, err)

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
	bulkRequestRedefine := gg.esClient.Bulk()
	for _, esID := range esIDs {
		bulkRequest.Add(elastic.NewBulkDeleteRequest().Index(gg.cfg.ES.PWIndex).
			Type(gg.cfg.ES.PWType).Id(esID))

		bulkRequestRedefine.Add(elastic.NewBulkDeleteRequest().Index(gg.cfg.ES.PWIndexRedefine).
			Type(gg.cfg.ES.PWType).Id(esID))
	}
	if bulkRequest.NumberOfActions() != len(esIDs) {
		return fmt.Errorf("NumberOfActions[%d] != esIDs[%d]", bulkRequest.NumberOfActions(), len(esIDs))
	}
	_, res := bulkRequestRedefine.Do(context.Background())
	icelog.Info("批量删除结果：", res, esIDs)

	_, err := bulkRequest.Do(context.Background())
	return err
}

func (gg *GodGame) ESUpdateGodGame(id string, data map[string]interface{}) error {
	_, err := gg.esClient.Update().Index(gg.cfg.ES.PWIndex).Type(gg.cfg.ES.PWType).
		Id(id).Doc(data).Do(context.Background())

	_, res := gg.esClient.Update().Index(gg.cfg.ES.PWIndexRedefine).Type(gg.cfg.ES.PWType).
		Id(id).Doc(data).Do(context.Background())
	icelog.Info("修改ES数据结果 ：", err, res, data, id)

	if err != nil {
		return err
	}
	return nil
}

func (gg *GodGame) ESDeleteGodGame(id string) error {
	_, err := gg.esClient.Delete().Index(gg.cfg.ES.PWIndex).Type(gg.cfg.ES.PWType).
		Id(id).
		Do(context.Background())

	_, res := gg.esClient.Delete().Index(gg.cfg.ES.PWIndexRedefine).Type(gg.cfg.ES.PWType).
		Id(id).
		Do(context.Background())

	icelog.Info("删除结果：", err, res, id)

	if err != nil {
		return err
	}
	return nil
}

// 组装Elasticsearch一行陪玩数据
func (gg *GodGame) BuildESGodGameData(godID, gameID int64) (model.ESGodGameRedefine, error) {
	var result model.ESGodGameRedefine
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
	result.LFO = time.Unix(sortResp.GetData().GetLfo(), 0)
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
	if godGame.PeiwanPriceType == constants.PW_PRICE_TYPE_BY_OM {
		result.Price = godGame.PeiwanPrice
	} else {
		result.PriceID = accpetOrderSetting.PriceID
	}
	result.HighestLevelID = godGame.HighestLevelID
	if godGame.Video != "" || godGame.Videos != "" {
		result.Video = 1
	}
	return result, nil
}

// 重构ES数据 godgame
func (gg *GodGame) BuildESGodGameDataRedefine(godID, gameID int64) (model.ESGodGameRedefine, error) {
	var result model.ESGodGameRedefine

	if godID == 0 || gameID == 0 {
		return result, fmt.Errorf("get god info error %d-%d", godID, gameID)
	}

	privacyInfo, _ := userpb.GetPrivacyCfg(frame.TODO(), &userpb.GetPrivacyCfgReq{
		UserId: godID,
	})
	var IsShowNear = int64(2)
	if privacyInfo.GetData() != nil {
		IsShowNear = 2
	} else {
		IsShowNear = privacyInfo.GetData().GetIsShowNear()
	}

	if data, err := gg.BuildESGodGameData(godID, gameID); err == nil {
		geoInfo, geoErr := userpb.Location(frame.TODO(), &userpb.LocationReq{
			UserId: data.GodID,
		})
		result.GodID = data.GodID
		result.GameID = data.GameID
		result.Gender = data.Gender
		result.PeiWanStatus = data.PeiWanStatus
		result.LTS = time.Now()
		result.LFO = data.LFO
		result.OrderCnt = data.OrderCnt
		result.SevenDaysCnt = data.SevenDaysCnt
		result.SevenDaysHours = data.SevenDaysHours
		result.RejectOrder = data.RejectOrder
		result.Weight = data.Weight
		result.PassedTime = data.PassedTime
		result.PriceID = data.PriceID
		result.Price = data.Price
		result.HighestLevelID = data.HighestLevelID
		result.IsShowNear = IsShowNear
		if geoErr == nil && geoInfo.GetErrcode() == 0 {
			result.City = geoInfo.GetData().GetCity()
			result.District = geoInfo.GetData().GetDistrict()
			result.Location2 = elastic.GeoPointFromLatLon(geoInfo.GetData().GetLat(), geoInfo.GetData().GetLng())
		}
		result.Video = data.Video
	}

	godGame := gg.dao.GetGodGame(godID, gameID)

	counts, err := plorderpb.Count(frame.TODO(), &plorderpb.CountReq{
		GodId:  godID,
		GameId: gameID,
	})
	if err == nil {
		result.OrderSetCnt = counts.GetData().GetCompletedHoursAmount()
	}
	result.GodLevel = godGame.GodLevel
	result.IsVoice = 0
	// 语聊品类不展示
	if gg.isVoiceCallGame(gameID) {
		result.IsVoice = 1
	}

	icelog.Info("大神品类更新: ", result)
	return result, nil
}

func (gg *GodGame) updateESGodGameRedefine() model.ESGodGameRedefine {
	var result model.ESGodGameRedefine

	return result
}

// 刷新全部大神池
func (gg *GodGame) FlashAllGods1(c frame.Context) error {
	var req godgamepb.FlashAllQuickOrderReq
	// 刷新单个大神
	if err := c.Bind(&req); err == nil && req.GodId > 0 {
		lists, err := gg.dao.GetGodAcceptSettings(req.GodId)
		if err == nil && len(lists) > 0 {
			for _, v := range lists {
				var data model.ESGodGameRedefine
				data, err := gg.BuildESGodGameDataRedefine(v.GodID, v.GameID)
				if err != nil {
					return c.RetBadRequestError(err.Error())
				}
				gg.ESAddGodGameInternal(data)
			}
			return c.RetSuccess("success", nil)
		}
	}

	// 刷新全部大神 及品类
	lists, err := gg.dao.GetGodsAcceptSettings()
	if err == nil && len(lists) > 0 {
		for _, v := range lists {
			// 更新位置数据
			geoInfo, geoErr := userpb.Location(frame.TODO(), &userpb.LocationReq{
				UserId: v.GodID,
			})
			if geoErr == nil {
				gg.ESUpdateGodGame(fmt.Sprintf("%d-%d", v.GodID, v.GameID), map[string]interface{}{
					"location2": elastic.GeoPointFromLatLon(geoInfo.GetData().GetLat(), geoInfo.GetData().GetLng()),
				})
			}
		}
		return c.RetSuccess("success", nil)
	}
	return c.RetSuccess("没有大神开启急速接单", nil)

}

// 刷新全部大神池
func (gg *GodGame) FlashAllGods(c frame.Context) error {
	var req godgamepb.FlashAllGodsReq
	if err := c.Bind(&req); err == nil {
		// 刷新单个大神 及品类
		if req.GetGodId() > 0 {
			go func() {
				lists, err := gg.dao.GetGodAcceptSettings(req.GodId)
				if err == nil && len(lists) > 0 {
					for _, v := range lists {
						var data model.ESGodGameRedefine
						data, err := gg.BuildESGodGameDataRedefine(v.GodID, v.GameID)
						if err != nil {
							return
						}
						gg.ESAddGodGameInternal(data)
					}
					return
				}
			}()
			return c.RetSuccess("success 已经异步刷新大神池，请不要频繁操作", nil)
		}

		// 刷新全部大神 及品类  标识game==100
		if req.GetTag() == 100 {
			go func() {
				lists, err := gg.dao.GetGodsAcceptSettings()
				if err == nil && len(lists) > 0 {
					for _, v := range lists {
						var data model.ESGodGameRedefine
						data, err := gg.BuildESGodGameDataRedefine(v.GodID, v.GameID)
						if err != nil {
							return
						}
						gg.ESAddGodGameInternal(data)
					}
					return
				}

			}()
			return c.RetSuccess("success 已经异步刷新大神池，请不要频繁操作", nil)
		}

		if req.GetTag() == 50 {
			go func() {
				lists, err := gg.dao.GetGodsAcceptSettings()
				if err == nil && len(lists) > 0 {
					for _, v := range lists {
						// 更新位置数据
						geoInfo, geoErr := userpb.Location(frame.TODO(), &userpb.LocationReq{
							UserId: v.GodID,
						})
						if geoErr == nil {
							gg.ESUpdateGodGame(fmt.Sprintf("%d-%d", v.GodID, v.GameID), map[string]interface{}{
								"location2": elastic.GeoPointFromLatLon(geoInfo.GetData().GetLat(), geoInfo.GetData().GetLng()),
							})
						}
					}
					return
				}

			}()
			return c.RetSuccess("success 已经异步刷新大神池，请不要频繁操作", nil)
		}

	}

	return c.RetSuccess("没有大神开启急速接单", nil)
}
