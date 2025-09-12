package services

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ====== JWT 配置 ======
const (
	jwtCookieName = "qtk" // 你的站内访问票据 Cookie 名
)

// 强烈建议改为从配置/ENV 注入
var hmacSecret = []byte(os.Getenv("CF_TURNSTILE_SECRET"))

type AccessClaims struct {
	AllowKeyList []string `json:"allowKeyList"`
	Scope        string   `json:"scope"` // 可选: 比如 "page:view"
	jwt.RegisteredClaims
}

// ReadTokenFromRequest 工具：从 Cookie / Authorization 里取出 token
func ReadTokenFromRequest(c *gin.Context) string {
	// 1) Cookie 优先
	if ck, err := c.Request.Cookie(jwtCookieName); err == nil && ck.Value != "" {
		return ck.Value
	}
	// 2) Authorization: Bearer
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// ParseClaims 工具：解析 token -> claims（容错：解析失败返回空 claims）
func ParseClaims(tok string) (*AccessClaims, error) {
	if tok == "" {
		return &AccessClaims{}, nil
	}
	parsed, err := jwt.ParseWithClaims(tok, &AccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Method)
		}
		return hmacSecret, nil
	})
	if err != nil || !parsed.Valid {
		return &AccessClaims{}, err
	}
	if cl, ok := parsed.Claims.(*AccessClaims); ok {
		return cl, nil
	}
	return &AccessClaims{}, fmt.Errorf("claims cast error")
}

// AppendAllowKey 工具：将新的 key 追加到 allowKeyList（去重）
func AppendAllowKey(list []string, key string) []string {
	if key == "" {
		return list
	}
	if !slices.Contains(list, key) {
		list = append(list, key)
	}
	return list
}

// IssueCookie 工具：签发并写回 Cookie
func IssueCookie(c *gin.Context, claims *AccessClaims) error {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(hmacSecret)
	if err != nil {
		return err
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     jwtCookieName,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode, // 生产环境必须 https
		SameSite: http.SameSiteLaxMode,
		// 不设置 Expires/MaxAge -> 会话 Cookie；如需长期保存可设置 Expires
	})
	return nil
}
