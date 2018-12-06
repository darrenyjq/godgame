package main

import (
	"flag"
	"fmt"
	"iceberg/frame/config"
	log "iceberg/frame/icelog"
	"laoyuegou.com/version"
	"laoyuegou.pb/godgame/pb"
	"os"
	"path/filepath"
	"play/godgame/api"
	godGameCfg "play/godgame/config"
)

var (
	cfgFile     = flag.String("config-path", "godgame.json", "config file")
	logLevel    = flag.String("loglevel", "DEBUG", "log level")
	showVersion = flag.Bool("version", false, "print version string")
)

func main() {
	// 设置进程的当前目录为程序所在的路径
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	os.Chdir(dir)
	// 解析命令行参数
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Version("godgame"))
		return
	}
	var cfg godGameCfg.Config
	config.Parseconfig(*cfgFile, &cfg)
	log.SetLevel(*logLevel)
	cfg.LogLevel = *logLevel
	s := api.NewGodGame(cfg)
	godgamepb.RegisterGodGameServer(s, &cfg.Base)
}
