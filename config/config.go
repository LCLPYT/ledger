package config

import (
	"errors"
	"os"

	"github.com/spf13/viper"
)

func CreateCommonConfig() (*viper.Viper, error) {
	v := viper.New()

	v.AutomaticEnv()

	v.SetConfigFile(".env")
	v.SetConfigType("env")

	if err := v.MergeInConfig(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	return v, nil
}
