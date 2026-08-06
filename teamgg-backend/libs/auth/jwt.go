package auth

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt"
	"net/http"
	"os"
	"strings"
	"team.gg-server/libs/crypto"
	"team.gg-server/libs/db"
	"team.gg-server/util"
	"time"
)

type authToken struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type AuthTokenBundle struct {
	AccessToken  authToken `json:"access_token"`
	RefreshToken authToken `json:"refresh_token"`
}

func GetAccessTokenExpireDuration() (time.Duration, error) {
	jwtAccessExpireTimeRaw := os.Getenv("JWT_ACCESS_EXPIRE")
	jwtAccessExpireTime, err := util.ParseDuration(jwtAccessExpireTimeRaw)
	if err != nil {
		return 0, err
	}
	return jwtAccessExpireTime, nil
}

func GetRefreshTokenExpireDuration() (time.Duration, error) {
	jwtRefreshExpireTimeRaw := os.Getenv("JWT_REFRESH_EXPIRE")
	jwtRefreshExpireTime, err := util.ParseDuration(jwtRefreshExpireTimeRaw)
	if err != nil {
		return 0, err
	}
	return jwtRefreshExpireTime, nil
}

func CreateAuthToken(uid string) (*AuthTokenBundle, error) {
	var err error
	atd := &AuthTokenBundle{}

	if uid == "" {
		return nil, errors.New("uid empty")
	}

	// load jwt secret from env
	jwtAccessSecretKey := crypto.JwtAccessSecretKey
	jwtRefreshSecretKey := os.Getenv("JWT_REFRESH_SECRET")

	jwtAccessExpireTime, err := GetAccessTokenExpireDuration()
	if err != nil {
		return nil, err
	}
	jwtRefreshExpireTime, err := GetRefreshTokenExpireDuration()
	if err != nil {
		return nil, err
	}

	// set access token
	atd.AccessToken.ExpiresAt = time.Now().Add(jwtAccessExpireTime).Unix() // 3 hours expiration
	accessTokenClaims := jwt.MapClaims{}
	accessTokenClaims["uid"] = uid
	accessTokenClaims["exp"] = atd.AccessToken.ExpiresAt
	accessTokenClaims["authorized"] = true
	signedAccessClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	atd.AccessToken.Token, err = signedAccessClaims.SignedString([]byte(jwtAccessSecretKey))
	if err != nil {
		return nil, err
	}

	// set refresh token
	atd.RefreshToken.ExpiresAt = time.Now().Add(jwtRefreshExpireTime).Unix() // 7 days expiration
	refreshTokenClaims := jwt.MapClaims{}
	refreshTokenClaims["uid"] = uid
	refreshTokenClaims["exp"] = atd.RefreshToken.ExpiresAt
	signedRefreshClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	atd.RefreshToken.Token, err = signedRefreshClaims.SignedString([]byte(jwtRefreshSecretKey))
	if err != nil {
		return nil, err
	}

	return atd, nil
}

func ValidateToken(rawToken string, secret string) (string, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(rawToken, jwt.MapClaims{})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("cannot parse claims")
	}

	userId, ok := claims["uid"].(string)
	if !ok || len(userId) == 0 {
		return "", errors.New("uid is empty or invalid")
	}

	_, err = jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		var ve *jwt.ValidationError
		ok := errors.As(err, &ve)
		if ok && ve.Errors == jwt.ValidationErrorExpired {
			return userId, ve
		}
		return "", err
	}

	return userId, nil
}

func SaveRefreshToken(uid string, refreshToken authToken) error {
	refreshTokenExpiresUnix := time.Unix(refreshToken.ExpiresAt, 0)
	now := time.Now()

	if err := db.InMemoryDB.SetExp(uid, refreshToken.Token, refreshTokenExpiresUnix.Sub(now)); err != nil {
		return err
	}
	return nil
}

func LoadRefreshToken(uid string) (string, error) {
	refreshToken, err := db.InMemoryDB.Get(uid)
	if err != nil {
		return "", err
	}
	return refreshToken, nil
}

func DeleteRefreshToken(uid string) error {
	if err := db.InMemoryDB.Del(uid); err != nil {
		return err
	}
	return nil
}

func ExtractAuthToken(req *http.Request) (string, error) {
	bearer := req.Header.Get("Authorization")
	token := strings.Split(bearer, " ")
	if len(bearer) == 0 {
		return "", errors.New("token not found in header")
	}
	if len(token) != 2 {
		return "", fmt.Errorf("invalid token: %s", bearer)
	}
	authToken := token[1]
	if len(authToken) == 0 {
		return "", errors.New("token empty")
	}
	return authToken, nil
}
