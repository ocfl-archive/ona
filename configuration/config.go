package configuration

import "github.com/je4/utils/v2/pkg/stashconfig"

type Config struct {
	Url       string             `yaml:"url" toml:"Url"`
	Key       string             `yaml:"key" toml:"Key"`
	ChunkSize int64              `yaml:"chunk-size" toml:"ChunkSize"`
	BarPause  int                `yaml:"bar-pause" toml:"BarPause"`
	StatusUrl string             `yaml:"status-url" toml:"StatusUrl"`
	JwtKey    string             `yaml:"jwt-key" toml:"JwtKey"`
	Storage   Storage            `yaml:"storage" toml:"storage"`
	Log       stashconfig.Config `yaml:"log" toml:"Log"`
}

type Storage struct {
	Type         string `yaml:"type" toml:"type"`
	Name         string `yaml:"name" toml:"name"`
	Key          string `yaml:"key" toml:"key"`
	Secret       string `yaml:"secret" toml:"secret"`
	ApiUrlValue  string `yaml:"api-url-value" toml:"apiurlvalue"`
	UploadFolder string `yaml:"upload-folder" toml:"uploadfolder"`
	Url          string `yaml:"url" toml:"url"`
	CAPEM        string `yaml:"capem" toml:"capem"`
	Debug        bool   `yaml:"debug" toml:"debug"`
}
