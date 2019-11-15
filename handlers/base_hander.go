package handlers

import (
	"context"
	"github.com/json-iterator/go"
	"godgame/config"
	"godgame/core"
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
			NsqWriters: self.cfg.Nsq.Writers,
			NsqLookups: self.cfg.Nsq.Lookups,
		}
		godGameImOnline.Init2(self.ctx, self.cfg.Nsq.ImTopic, "godgame_time", &GodImOnline{self.dao})
	})
}

func (self *BaseHandler) Stop() {
	self.exit()
	self.waitGroup.Wait()
}
