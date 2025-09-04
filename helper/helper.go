package helper

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/mileusna/useragent"
)

// 统一渲染：自动把 SiteKey 合并到模板数据里
func RenderHTML(c *gin.Context, status int, tmpl string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data["SiteKey"] = os.Getenv("CF_TURNSTILE_SITEKEY")
	c.HTML(status, tmpl, data)
}
func HashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// 使用 crypto/rand 生成不可预测 key；字符集避免易混淆字符
func RandKey(length int) (string, error) {
	const al = "ABCDEFHJKLMNPQRSTWXY123456789" // 31 chars
	var out = make([]byte, length)
	max := big.NewInt(int64(len(al)))
	for i := 0; i < length; i++ {
		r, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = al[r.Int64()]
	}
	return string(out), nil
}

func ParseUA(userAgent string) useragent.UserAgent {
	fmt.Println(userAgent)
	fmt.Println(useragent.Parse(userAgent))
	return useragent.Parse(userAgent)
}
