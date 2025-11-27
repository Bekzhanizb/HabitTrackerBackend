package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *zap.Logger

func InitLogger() {
	writer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "./logs/app.log",
		MaxSize:    50, // MB
		MaxBackups: 7,
		MaxAge:     14, // days
		Compress:   true,
	})

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoder := zapcore.NewJSONEncoder(encoderCfg)

	core := zapcore.NewCore(encoder, writer, zap.InfoLevel)
	Logger = zap.New(core, zap.AddCaller())
}
