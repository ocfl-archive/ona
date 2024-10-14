package configuration

import "github.com/je4/utils/v2/pkg/stashconfig"

type Config struct {
	Url       string             `yaml:"url" toml:"Url"`
	Key       string             `yaml:"key" toml:"Key"`
	ChunkSize int64              `yaml:"chunk-size" toml:"ChunkSize"`
	BarPause  int                `yaml:"bar-pause" toml:"BarPause"`
	StatusUrl string             `yaml:"status-url" toml:"StatusUrl"`
	JwtKey    string             `yaml:"jwt-key" toml:"JwtKey"`
	Log       stashconfig.Config `yaml:"log" toml:"Log"`
}
