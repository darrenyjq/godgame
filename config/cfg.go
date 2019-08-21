package config

import (
	"iceberg/frame/config"
	"time"
)

type oss struct {
	OSSAccessID  string `json:"oss_accessid"`
	OSSAccessKey string `json:"oss_accesskey"`
}

type es struct {
	Host            []string `json:"host"`
	PWIndex         string   `json:"pw_index"`
	PWIndexRedefine string   `json:"pw_index_redefine"`
	PWType          string   `json:"pw_type"`
	Username        string   `json:"username,omitempty"`
	Password        string   `json:"password,omitempty"`
}

type shence struct {
	Timeout int    `json:"timeout"`
	URL     string `json:"url"`
	Project string `json:"project"`
}

type nsq struct {
	Topic   string   `json:"nsq_topic"`
	Writers []string `json:"nsq_writers"`
	Lookups []string `json:"nsq_lookups"`
}

type Config struct {
	Env                 config.Environment `json:"env"`
	Base                config.BaseCfg     `json:"baseCfg"`
	Mysql               config.MysqlCfg    `json:"mysqlCfg"`
	Redis               config.RedisCfg    `json:"redisCfg"`
	ES                  es                 `json:"es"`
	OSS                 oss                `json:"oss"`
	Urls                map[string]string  `json:"urls"`
	Mix                 map[string]string  `json:"mix"`
	Shence              shence             `json:"shence"`
	GodLTSDuration      int                `json:"god_lts_duration"`
	YunPianApiKey       string             `json:"yunpian_apikey"`
	Nsq                 nsq                `json:"nsq"`
	FillGodListInterval time.Duration      `json:"fill_god_list_interval"`
}
