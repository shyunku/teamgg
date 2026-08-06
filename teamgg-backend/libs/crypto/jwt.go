package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

var (
	JwtAccessSecretKey  = ""
	JwtRefreshSecretKey = ""
)

func Initialize() {
	JwtAccessSecretKey = os.Getenv("JWT_ACCESS_SECRET")
	JwtRefreshSecretKey = os.Getenv("JWT_REFRESH_SECRET")
}

func generateJWTSecretKey(length int) string {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

func PrintNewJwtSecret() {
	key := generateJWTSecretKey(32)
	fmt.Println(key)
}
