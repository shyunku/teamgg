package middlewares

import (
	"github.com/gin-gonic/gin"
	log "github.com/shyunku-libraries/go-logger"
	"net/http"
	"strings"
	util2 "team.gg-server/controllers/util"
	"team.gg-server/libs/auth"
	"team.gg-server/libs/crypto"
	"team.gg-server/util"
)

func AuthMiddleware(c *gin.Context) {
	accessToken, err := c.Cookie("accessToken")
	if err != nil {
		log.Warn(err)
		util.AbortWithErrJson(c, http.StatusUnauthorized, err)
		return
	}

	uid, err := auth.ValidateToken(accessToken, crypto.JwtAccessSecretKey)
	if err != nil {
		log.Warn(err)

		// try to refresh token
		if len(uid) == 0 {
			log.Error("uid is empty")
			util.AbortWithErrJson(c, http.StatusUnauthorized, err)
			return
		}

		refreshToken, err := auth.LoadRefreshToken(uid)
		if err != nil {
			log.Warn(err)
			util.AbortWithErrJson(c, http.StatusUnauthorized, err)
			return
		}

		// validate refresh token
		refreshTokenUserId, err := auth.ValidateToken(refreshToken, crypto.JwtRefreshSecretKey)
		if err != nil {
			log.Warn(err)
			util.AbortWithErrJson(c, http.StatusUnauthorized, err)
			return
		}
		if uid != refreshTokenUserId {
			// invalid refresh token
			log.Warn(err)
			util.AbortWithErrJson(c, http.StatusUnauthorized, err)
			return
		}

		log.Infof("refreshing token for user %s", uid)

		// delete refresh token
		if err := auth.DeleteRefreshToken(uid); err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusUnauthorized, "internal server error")
			return
		}

		// create auth token
		authTokenBundle, err := auth.CreateAuthToken(uid)
		if err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusUnauthorized, "internal server error")
			return
		}

		// save refresh token to in-memory
		if err := auth.SaveRefreshToken(uid, authTokenBundle.RefreshToken); err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusUnauthorized, "failed to save refresh token")
			return
		}

		// save on cookie
		refreshTokenExpireDuration, err := auth.GetRefreshTokenExpireDuration()
		if err != nil {
			log.Error(err)
			util.AbortWithStrJson(c, http.StatusUnauthorized, "internal server error")
			return
		}
		util2.SetAccessTokenCookie(c, authTokenBundle.AccessToken.Token, int(refreshTokenExpireDuration.Seconds()))
	}

	c.Set("uid", uid)
	c.Request.Header.Set("uid", uid)
	c.Next()
}

func UnsafeAuthMiddleware(c *gin.Context) {
	authz := c.GetHeader("Authorization") // "Bearer <token>"
	var accessToken string
	if strings.HasPrefix(authz, "Bearer ") {
		accessToken = strings.TrimPrefix(authz, "Bearer ")
	} else {
		// 보조: 쿠키도 시도 (파폭 데스크톱 임시 호환)
		if v, err := c.Cookie("accessToken"); err == nil {
			accessToken = v
		}
	}

	defer func() {
		c.Next()
	}()

	if accessToken == "" {
		return
	}

	log.Infof("access_token: %s", accessToken)

	uid, err := auth.ValidateToken(accessToken, crypto.JwtAccessSecretKey)
	if err != nil {
		// try to refresh token
		if len(uid) == 0 {
			return
		}

		refreshToken, err := auth.LoadRefreshToken(uid)
		if err != nil {
			return
		}

		// validate refresh token
		refreshTokenUserId, err := auth.ValidateToken(refreshToken, crypto.JwtRefreshSecretKey)
		if err != nil {
			return
		}
		if uid != refreshTokenUserId {
			// invalid refresh token
			return
		}

		log.Infof("refreshing token for user %s", uid)

		// delete refresh token
		if err := auth.DeleteRefreshToken(uid); err != nil {
			return
		}

		// create auth token
		authTokenBundle, err := auth.CreateAuthToken(uid)
		if err != nil {
			return
		}

		// save refresh token to in-memory
		if err := auth.SaveRefreshToken(uid, authTokenBundle.RefreshToken); err != nil {
			return
		}

		// save on cookie
		refreshTokenExpireDuration, err := auth.GetRefreshTokenExpireDuration()
		if err != nil {
			return
		}
		util2.SetAccessTokenCookie(c, authTokenBundle.AccessToken.Token, int(refreshTokenExpireDuration.Seconds()))
	}

	c.Set("uid", uid)
	c.Request.Header.Set("uid", uid)
}

func CORSMiddleware(allowed map[string]struct{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With") // <-- Authorization

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
