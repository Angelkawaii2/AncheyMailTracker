package helper

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewZap() *zap.Logger {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	rotate := &lumberjack.Logger{
		Filename:   "/var/log/app/access.log",
		MaxSize:    100, // MB
		MaxBackups: 7,
		MaxAge:     14, // days
		Compress:   true,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(rotate)),
		zap.InfoLevel,
	)
	return zap.New(core, zap.AddCaller())
}

func AccessLogZap(l *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		l.Info("http_access",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("route", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.String("ip", c.ClientIP()),
			zap.String("ua", c.Request.UserAgent()),
			zap.Duration("latency", time.Since(start)),
			zap.String("rid", c.GetString("rid")),
			zap.Int("bytes_out", c.Writer.Size()),
			zap.Int64("bytes_in", c.Request.ContentLength),
			zap.String("referer", c.Request.Referer()),
			zap.String("host", c.Request.Host),
			//zap.Array("errors", c.Errors.Errors()), // gin collected errors
		)
	}
}
