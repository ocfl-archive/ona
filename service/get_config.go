package service

import (
	"fmt"
	"github.com/joho/godotenv"
	"ona/configuration"
	"os"
	"strconv"
)

func GetConfig() *configuration.Config {
	err := godotenv.Load(".env")
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
