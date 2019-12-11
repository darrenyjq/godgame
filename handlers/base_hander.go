package handlers

import (
	"context"
	"github.com/json-iterator/go"
	"godgame/config"
	"godgame/core"
	"iceberg/frame/icelog"
	"laoyuegou.com/mq"
	"laoyuegou.com/util"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type BaseHandler struct {
	cfg       config.Config
	dao       *core.Dao
	waitGroup util.WaitGroupWrapper
	exit      context.CancelFunc
	ctx       context.Context
}

func NewBaseHandler(cfg config.Config, dao *core.Dao) *BaseHandler {
	ctx, exit := context.WithCancel(context.Background())
	h := &BaseHandler{
		cfg:  cfg,
		dao:  dao,
		exit: exit,
		ctx:  ctx,
	}
	h.init()
	return h
}

func (self *BaseHandler) init() {
	self.waitGroup.Wrap(func() {
		godLevelConsumer := &mq.NsqConsumer{
			NsqWriters: self.cfg.Nsq.Writers,
			NsqLookups: self.cfg.Nsq.Lookups,
		}
		godLevelConsumer.Init2(self.ctx, self.cfg.Nsq.Topic, "god_level", &GodLevelHandler{self.dao})
	})

	self.waitGroup.Wrap(func() {
		godGameImOnline := &mq.NsqConsumer{
			NsqWriters: self.cfg.IMNsq.Writers,
			NsqLookups: self.cfg.IMNsq.Lookups,
		}
		icelog.Info("启动大神上下线事件监控")
		godGameImOnline.Init2(self.ctx, self.cfg.Nsq.Topic, "godgame_time", &GodImOnline{self.dao})
	})

	// 私聊自动回复问题，后面再做
	self.waitGroup.Wrap(func() {
		messageRespConsumer := &mq.NsqConsumer{
			NsqWriters: self.cfg.IMNsq.Writers,
			NsqLookups: self.cfg.IMNsq.Lookups,
		}
		icelog.Info("启动IM私聊监控")
		messageRespConsumer.Init2(self.ctx, "message", "godgame_auto_grab_order", &AutoGrabOrderHandler{self.dao})
	})

}

func (self *BaseHandler) Stop() {
	self.exit()
	self.waitGroup.Wait()
}
