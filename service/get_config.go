package service

import (
	"github.com/jinzhu/configor"
	"log"
	"ona/configuration"
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

	return &configObj
}
