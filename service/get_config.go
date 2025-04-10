package service

import (
	"github.com/je4/filesystem/v3/pkg/vfsrw"
	"github.com/je4/utils/v2/pkg/config"
	"github.com/jinzhu/configor"
	"github.com/ocfl-archive/ona/configuration"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

func GetConfig(cfgFilePathRaw string) *configuration.Config {

	configObj := configuration.Config{}
	if cfgFilePathRaw != "" {
		cfgFilePath := filepath.ToSlash(filepath.Clean(cfgFilePathRaw))
		err := configor.Load(&configObj, cfgFilePath)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		configObj = configuration.Config{
			Url:       os.Getenv("URL"),
			Key:       os.Getenv("KEY"),
			JwtKey:    os.Getenv("JWT_KEY"),
			StatusUrl: os.Getenv("STATUS_URL"),
		}
		chunkSize, _ := strconv.Atoi(os.Getenv("CHUNK_SIZE"))
		configObj.ChunkSize = int64(chunkSize)
		configObj.BarPause, _ = strconv.Atoi(os.Getenv("BAR_PAUSE"))
	}
	if configObj.Log.Level == "" {
		configObj.Log.Level = "INFO"
	}
	if configObj.Storage.Secret == "" {
		configObj.Storage.Secret = os.Getenv("SECRET")
	}
	return &configObj
}

func LoadVfsConfig(cfg configuration.Config) (vfsrw.Config, error) {
	vfsTemp := vfsrw.VFS{
		Type: cfg.Storage.Type,
		Name: cfg.Storage.Name,
		S3: &vfsrw.S3{
			AccessKeyID:     config.EnvString(cfg.Storage.Key),
			SecretAccessKey: config.EnvString(cfg.Storage.Secret),
			Endpoint:        config.EnvString(cfg.Storage.Url),
			Region:          "us-east-1",
			UseSSL:          true,
			Debug:           cfg.Storage.Debug,
			CAPEM:           cfg.Storage.CAPEM,
		},
	}

	vfsMap := make(map[string]*vfsrw.VFS)
	vfsMap[cfg.Storage.Name] = &vfsTemp
	return vfsMap, nil
}
