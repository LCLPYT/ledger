package config

import "github.com/spf13/viper"

func CreateCommonConfig() *viper.Viper {
	v := viper.New()

	v.SetEnvPrefix("ledger")
	v.AutomaticEnv()

	return v
}
