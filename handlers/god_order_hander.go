package handlers

import (
	"github.com/nsqio/go-nsq"
	"godgame/core"
	"iceberg/frame/icelog"
)

type GodOrderHandler struct {
	dao *core.Dao
}

// 订单完成、新的评价触发重新计算大神等级
func (self *GodOrderHandler) HandleMessage(msg *nsq.Message) error {
	icelog.Errorf("GodOrderHandler received: %s", msg.Body)
	icelog.Errorf("this  is new  received")
	icelog.Infof("test one")
	return nil

}
