package config

import (
	"iceberg/frame/config"
)

type Config struct {
	Env   config.Environment `json:"env"`
	Base  config.BaseCfg     `json:"baseCfg"`
	Mysql config.MysqlCfg    `json:"mysqlCfg"`
	Redis config.RedisCfg    `json:"redisCfg"`
	ES    struct {
		Host     []string `json:"host"`
		PWIndex  string   `json:"pw_index"`
		PWType   string   `json:"pw_type"`
		Username string   `json:"username,omitempty"`
		Password string   `json:"password,omitempty"`
	} `json:"es"`
	OSS struct {
		OSSAccessID  string `json:"oss_accessid"`
		OSSAccessKey string `json:"oss_accesskey"`
	} `json:"oss"`
	Urls   map[string]string `json:"urls"`
	Mix    map[string]string `json:"mix"`
	Shence struct {
		Timeout int    `json:"timeout"`
		URL     string `json:"url"`
		Project string `json:"project"`
	} `json:"shence"`
	GodLTSDuration int    `json:"god_lts_duration"`
	YunPianApiKey  string `json:"yunpian_apikey"`
}
