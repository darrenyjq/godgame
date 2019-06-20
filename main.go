package main

import (
	"flag"
	"fmt"
	"godgame/api"
	godGameCfg "godgame/config"
	"iceberg/frame/config"
	"iceberg/frame/icelog"
	"laoyuegou.com/version"
	"laoyuegou.pb/godgame/pb"
	"os"
	"path/filepath"
)

var (
	cfgFile     = flag.String("config-path", "godgame.json", "config file")
	logLevel    = flag.String("loglevel", "DEBUG", "log level")
	showVersion = flag.Bool("version", false, "print version string")
)

func main() {
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	os.Chdir(dir)
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Version("godgame"))
		return
	}
	var cfg godGameCfg.Config
	config.Parseconfig(*cfgFile, &cfg)
	if cfg.FillGodListInterval == 0 {
		cfg.FillGodListInterval = 60
	}
	icelog.SetLevel(*logLevel)
	s := api.NewGodGame(cfg)
	godgamepb.RegisterGodGameServer(s, &cfg.Base)
}
