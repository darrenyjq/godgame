package core

import (
	"encoding/json"
	"github.com/gomodule/redigo/redis"
	"laoyuegou.pb/godgame/pb"
)

func (dao *Dao) GetTimeLine() ([]*godgamepb.FeedsResp_IndexFeedRespData_FeedObj, error) {
	var feeds []*godgamepb.FeedsResp_IndexFeedRespData_FeedObj
	c := dao.cpool.Get()
	defer c.Close()
	bs, err := redis.Bytes(c.Do("GET", RKFeedTimeLine()))
	if err != nil {
		return feeds, err
	}
	err = json.Unmarshal(bs, &feeds)
	return feeds, err
}

func (dao *Dao) ResetTimeLine(p string) error {
	c := dao.cpool.Get()
	defer c.Close()
	_, err := c.Do("SET", RKFeedTimeLine(), p)
	return err
}
