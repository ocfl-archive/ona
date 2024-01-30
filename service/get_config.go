package service

import (
	"fmt"
	"github.com/joho/godotenv"
	"ona/configuration"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func GetConfig() *configuration.Config {
	_, b, _, _ := runtime.Caller(0)
	d1 := strings.Replace(filepath.ToSlash(path.Join(path.Dir(b))), "/service", "", -1)
	err := godotenv.Load(d1 + "/.env")
	if err != nil {
		fmt.Println(err)
	}
	configObj := &configuration.Config{
		Url:       os.Getenv("URL"),
		Key:       os.Getenv("KEY"),
		JwtKey:    os.Getenv("JWT_KEY"),
		StatusUrl: os.Getenv("STATUS_URL"),
	}
	chunkSize, _ := strconv.Atoi(os.Getenv("CHUNK_SIZE"))
	configObj.ChunkSize = int64(chunkSize)
	configObj.BarPause, _ = strconv.Atoi(os.Getenv("BAR_PAUSE"))
	return configObj
}
