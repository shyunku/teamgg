package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/shyunku-libraries/go-logger"
	"net/http"
	util2 "team.gg-server/controllers/util"
	"team.gg-server/libs/auth"
	"team.gg-server/libs/crypto"
	"team.gg-server/libs/db"
	"team.gg-server/models"
	"team.gg-server/util"
)

func UseAuthRouter(r *gin.RouterGroup) {
	g := r.Group("/auth")

	g.POST("/login", Login)
	g.POST("/signup", Signup)
	g.POST("/refresh", Refresh)
	g.POST("/logout", Logout)
	g.POST("/rso/start", RsoStart)
	g.POST("/rso/link/start", RsoLinkStart)
	g.GET("/rso/status", RsoStatus)
	g.POST("/rso/complete/existing", RsoCompleteExisting)
	g.POST("/rso/complete/new", RsoCompleteNew)
	g.GET("/rsoLogin", RsoLogin)
	g.GET("/rsoLogout", RsoLogout)
	g.GET("/me", GetMyAccount)
	g.DELETE("/me/riot", UnlinkRiotAccount)
	g.PUT("/me/riot/primary", SetPrimaryRiotAccount)
}

func Login(c *gin.Context) {
	var req LoginRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}

	comparablePw := util.Sha256(req.UserId + req.EncryptedPassword)
	userDAO, exists, err := models.GetUserDAO_byUserId_withPassword(db.Root, req.UserId, comparablePw)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if !exists {
		util.AbortWithStrJson(c, http.StatusUnauthorized, "user not found")
		return
	}

	resp, err := createLoginResponse(userDAO, userDAO.UserId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "failed to create auth token")
		return
	}
	c.JSON(http.StatusOK, resp)
}

func Signup(c *gin.Context) {
	var req SignupRequestDto
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusBadRequest, "invalid request")
		return
	}
	if len(req.UserId) < 4 {
		util.AbortWithStrJson(c, http.StatusBadRequest, "user id length must be greater than 4")
		return
	}

	_, exists, err := models.GetUserDAO_byUserId(db.Root, req.UserId)
	if err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	if exists {
		util.AbortWithStrJson(c, http.StatusConflict, "user already exists")
		return
	}

	userDAO := models.UserDAO{
		Uid:               uuid.New().String(),
		UserId:            req.UserId,
		EncryptedPassword: util.Sha256(req.UserId + req.EncryptedPassword),
	}
	if err := userDAO.Upsert(db.Root); err != nil {
		log.Error(err)
		util.AbortWithStrJson(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, nil)
}

func createLoginResponse(userDAO *models.UserDAO, displayUserId string) (*LoginResponseDto, error) {
	authTokenBundle, err := auth.CreateAuthToken(userDAO.Uid)
	if err != nil {
		return nil, err
	}
	if err := auth.SaveRefreshToken(userDAO.Uid, authTokenBundle.RefreshToken); err != nil {
		return nil, err
	}
	return &LoginResponseDto{
		Uid:          userDAO.Uid,
		UserId:       displayUserId,
		AccessToken:  authTokenBundle.AccessToken.Token,
		RefreshToken: authTokenBundle.RefreshToken.Token,
		ExpiresIn:    int(authTokenBundle.AccessToken.ExpiresAt),
	}, nil
}

func Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	uid, err := auth.ValidateToken(req.RefreshToken, crypto.JwtRefreshSecretKey)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	saved, err := auth.LoadRefreshToken(uid)
	if err != nil || saved != req.RefreshToken {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	bundle, err := auth.CreateAuthToken(uid)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if err := auth.SaveRefreshToken(uid, bundle.RefreshToken); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"accessToken":  bundle.AccessToken.Token,
		"refreshToken": bundle.RefreshToken.Token,
		"expiresIn":    int(bundle.AccessToken.ExpiresAt),
	})
}

func Logout(c *gin.Context) {
	util2.DeleteAccessTokenCookie(c)
	c.JSON(http.StatusOK, nil)
}

func RsoLogout(c *gin.Context) {
	c.JSON(http.StatusOK, "ok")
}
