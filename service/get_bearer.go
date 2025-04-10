package service

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ocfl-archive/ona/configuration"
	"time"
)

func GetBearer(config configuration.Config) (string, error) {

	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)

	claims["exp"] = time.Now().Add(time.Minute * 100).Unix()
	secret := []byte(config.JwtKey)
	tokenStr, err := token.SignedString(secret)

	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	return "Bearer " + tokenStr, nil
}
