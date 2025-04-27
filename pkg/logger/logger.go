package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

func Init(debug bool) {
	if debug {
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		var err error
		Log, err = config.Build()
		if err != nil {
			panic("failed to initialize logger: " + err.Error())
		}
	} else {
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.FatalLevel + 1)

		var err error
		Log, err = config.Build()
		if err != nil {
			panic("failed to initialize logger: " + err.Error())
		}
	}
}

func SetLogger(l *zap.Logger) {
	Log = l
}

func GetLogger() *zap.Logger {
	if Log == nil {
		Init(false)
	}
	return Log
}

func WithField(key string, value interface{}) *zap.Logger {
	if Log == nil {
		panic("logger not initialized")
	}
	return Log.With(zap.Any(key, value))
}
