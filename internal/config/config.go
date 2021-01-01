package config

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var conf Config

type (
	Config struct {
		loaded                bool
		LogLevel              string
		DatabaseFile          string
		SocketPath            string
		CheckInterval         uint
		Delay                 uint
		AppleId               string
		AppleIdHashedPassword string
	}
)

func ParseConfig(configName, configPath string) {
	viper.SetConfigType("yaml")
	viper.SetConfigName("xapsd")
	viper.SetConfigName(configName)
	viper.AddConfigPath("/etc/xapsd/")
	viper.AddConfigPath(configPath)

	// // Oprional
	// viper.SetDefault("LogLevel", "warn")
	// viper.SetDefault("DatabaseFile", "/var/lib/xapsd/database.json")
	// viper.SetDefault("SocketPath", "/var/run/dovecot/xapsd.sock")
	// viper.SetDefault("CheckInterval", 20)
	// viper.SetDefault("Delay", 30)
	// viper.SetDefault("AppleId", "apple@apple.com")
	// viper.SetDefault("AppleIdHashedPassword", "thehash")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}
	err = viper.Unmarshal(&conf)
	if err != nil {
		log.Fatal(err)
	}
	conf.loaded = true
}

func GetOptions() Config {
	if !conf.loaded {
		ParseConfig("", "")
	}
	return conf
}
