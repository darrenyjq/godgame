package api

import (
	"context"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"godgame/core"
	"gopkg.in/olivere/elastic.v5"
	"iceberg/frame"
	"iceberg/frame/icelog"
	"laoyuegou.pb/game/pb"
	"laoyuegou.pb/godgame/constants"
	godgamepb "laoyuegou.pb/godgame/pb"
	"strings"
	"time"
)

func (gg *GodGame) fetch_god_ids(game_id, gender int64, redisConn redis.Conn) {
	var keyByGender string
	now := time.Now()
	// searchService := gg.esClient.Scroll(gg.cfg.ES.PWIndex)
	searchService := gg.esClient.Scroll(gg.cfg.ES.PWIndexRedefine)
	query := elastic.NewBoolQuery().
		Must(elastic.NewRangeQuery("lts").Lte(now).Gte(now.AddDate(0, 0, gg.cfg.GodLTSDuration))).
		Must(elastic.NewTermQuery("game_id", game_id)).
		Should(elastic.NewMatchQuery("peiwan_status", "1").Boost(9)).
		Should(elastic.NewMatchQuery("peiwan_status", "2").Boost(5)).
		Should(elastic.NewMatchQuery("reject_order", "1").Boost(3)).
		Should(elastic.NewMatchQuery("reject_order", "2").Boost(6))
	if gender == constants.GENDER_MALE {
		keyByGender = core.RKGodListByGender(game_id, constants.GENDER_MALE)
		query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_FEMALE)).Boost(4)).
			Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_MALE)).Boost(7))
	} else {
		keyByGender = core.RKGodListByGender(game_id, constants.GENDER_FEMALE)
		query = query.Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_FEMALE)).Boost(7)).
			Should(elastic.NewMatchQuery("gender", fmt.Sprintf("%d", constants.GENDER_MALE)).Boost(4))
	}

	ctx := context.Background()
	searchService = searchService.Query(query).
		Sort("weight", false).
		Sort("_score", false).
		Sort("lts", false).
		Sort("seven_days_hours", false).
		Size(100)
	redisConn.Do("DEL", keyByGender)
	for {
		resp, err := searchService.Do(ctx)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
		} else {
			for _, item := range resp.Hits.Hits {
				if seq := strings.Split(item.Id, "-"); len(seq) == 2 {
					redisConn.Do("ZADD", keyByGender, *item.Score, seq[0])
				}
			}
		}
	}
}

func (gg *GodGame) FillGodList() {
	ticker := time.NewTicker(gg.cfg.FillGodListInterval * time.Second)
	redisKey := core.RKFillGodListLock()
	redisConn := gg.dao.GetRedisPool().Get()
	for {
		select {
		case <-ticker.C:
		case <-gg.exitChan:
			goto exit
		}
		icelog.Info("begin fill_god_list")
		if lock, _ := redis.String(redisConn.Do("SET", redisKey, "1", "NX", "EX", int(gg.cfg.FillGodListInterval))); lock != "OK" {
			icelog.Info("fill_god_list lock failed")
			continue
		}
		games, err := gamepb.GameInfos(frame.TODO(), nil)
		if err != nil || games.GetErrcode() != 0 {
			continue
		}
		for gid, _ := range games.GetData() {
			gg.fetch_god_ids(gid, constants.GENDER_MALE, redisConn)
			gg.fetch_god_ids(gid, constants.GENDER_FEMALE, redisConn)
		}
		redisConn.Do("DEL", redisKey)
		icelog.Info("finish fill_god_list")
	}
exit:
	icelog.Info("exiting fill_god_list loop...")
}

// 获取大神列表含带打折信息
func (gg *GodGame) GetGodsDiscount(c frame.Context) error {
	var req godgamepb.GetGodsDiscountReq
	if err := c.Bind(&req); err != nil {
		return c.RetForbiddenError(errParamMsg)
	} else if req.GetGameId() == 0 || len(req.GetGods()) == 0 {
		return c.RetForbiddenError(errParamMsg)
	}
	resp, err := gg.dao.GetGodsDiscount(req.GetGameId(), req.GetGods())
	if err != nil {
		return c.JSON2(ERR_CODE_BAD_REQUEST, err.Error(), nil)
	}
	return c.RetSuccess("success", resp)
}
