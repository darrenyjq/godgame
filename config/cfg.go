package config

import (
	"iceberg/frame/config"
)

type Config struct {
	LogLevel string          `json:"loglevel"`
	Env      string          `json:"env"`
	Mysql    config.MysqlCfg `json:"mysqlCfg"`
	Base     config.BaseCfg  `json:"baseCfg"`
	Redis    config.RedisCfg `json:"redisCfg"`

	MNS struct {
		URL            string `json:"url"`
		AccessID       string `json:"access_id"`
		AccessKey      string `json:"access_key"`
		QueueName      string `json:"queue_name"`
		PurseQueueName string `json:"purse_queue_name"`
	} `json:"mns"`

	ES struct {
		Host    []string `json:"host"`
		PWIndex string   `json:"pw_index"`
		PWType  string   `json:"pw_type"`
	} `json:"es"`

	IM struct {
		Addr     string `json:"addr"`
		AppID    string `json:"app_id"`
		AppToken string `json:"app_token"`
	} `json:"im"`

	Nsq struct {
		Topic   string   `json:"topic"`
		Writers []string `json:"writers"`
		Lookups []string `json:"lookups"`
	} `json:"nsq"`

	OSS struct {
		OSSAccessID  string `json:"oss_accessid"`
		OSSAccessKey string `json:"oss_accesskey"`
	} `json:"oss"`

	Urls map[string]string `json:"urls"`

	Mix map[string]string `json:"mix"`

	Shence struct {
		Timeout int    `json:"timeout"`
		URL     string `json:"url"`
		Project string `json:"project"`
	}

	GodLTSDuration int `json:"god_lts_duration"`
}
