package webprovider

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"zeusro.com/hermes/internal/core/config"
	"zeusro.com/hermes/internal/core/logprovider"
)

var (
	TokenExpired   = errors.New("过期Token")
	TokenMalformed = errors.New("无法解析Token")
	TokenInvalid   = errors.New("非法Token")
)

// todo: 自定义的用户声明
// YourUserClaims 自定义的用户声明
type YourUserClaims struct {
	UID   uint
	Email string
	jwt.RegisteredClaims
}

type JWT struct {
	SigningKey []byte
}

func (j *JWT) CreateToken(claims YourUserClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.SigningKey)
}

func (j *JWT) ParseToken(tokenString string) (*YourUserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &YourUserClaims{}, func(token *jwt.Token) (i interface{}, e error) {
		return j.SigningKey, nil
	})

	switch {
	case errors.Is(err, jwt.ErrTokenMalformed):
		return nil, TokenMalformed
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		return nil, TokenInvalid
	case errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet):
		return nil, TokenExpired
	}

	if token != nil {
		if claims, ok := token.Claims.(*YourUserClaims); ok && token.Valid {

			return claims, nil
		}
	}

	return nil, TokenInvalid
}

func (j *JWT) RefreshToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &YourUserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.SigningKey, nil
	})
	if err != nil {
		return "", err
	}
	if claims, ok := token.Claims.(*YourUserClaims); ok && token.Valid {
		claims.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(1 * time.Hour))
		claims.RegisteredClaims.IssuedAt = jwt.NewNumericDate(time.Now())

		return j.CreateToken(*claims)
	}
	return "", TokenInvalid
}

// --------------------------------------------------------------------------------------------------------------------------
type JWTMiddleware struct {
	l      logprovider.Logger
	gin    CorsMiddleware
	config config.Config
	JWT    JWT
	// userRepo repository.UserRepository
}

func NewJWTMiddleware(l logprovider.Logger,
	gin CorsMiddleware,
	config config.Config) JWTMiddleware {
	return JWTMiddleware{
		l:      l,
		gin:    gin,
		config: config,
		// userRepo: userRepo,
	}
}

func (m JWTMiddleware) SetUp() {}

// JWTAuth 普通用户验证
func (m JWTMiddleware) JWTAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		m.User(ctx)
	}
}
func (m JWTMiddleware) User(ctx *gin.Context) {
	token := ctx.Request.Header.Get("Authorization")
	if token == "" {
		// ctx.AbortWithStatusJSON(http.StatusUnauthorized, utils.ErrorResponse{
		// 	Code:    utils.NoToken,
		// 	Message: "未登录",
		// })
		return
	}

	if !strings.HasPrefix(token, "Bearer ") {
		// ctx.AbortWithStatusJSON(http.StatusBadRequest, utils.ErrorResponse{
		// 	Code:    utils.InvalidToken,
		// 	Message: "Token格式错误",
		// })
		return
	}

	token = strings.Split(token, " ")[1]

	j := JWT{SigningKey: []byte(m.config.JWT.SigningKey)}
	claims, err := j.ParseToken(token)
	if err != nil {
		// ctx.AbortWithStatusJSON(http.StatusUnauthorized, utils.ErrorResponse{
		// 	Code:    utils.NoToken,
		// 	Message: "未登录哦",
		// })
		return
	}

	ctx.Set("claims", claims)
	ctx.Set("uid", claims.UID)
	//ctx.Set("email", claims.Email)
	ctx.Next()
}

// Admin 管理员用户验证
func (m JWTMiddleware) Admin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		//todo: 这里可以添加其他的权限验证逻辑
		// result := m.AdminCheck(ctx)
		// if result.User == nil || !result.User.IsAdmin {
		// 	ctx.AbortWithStatusJSON(http.StatusUnauthorized, result.Response)
		// }
		ctx.Next()
	}
}
