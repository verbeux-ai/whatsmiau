package log_connect

import (
	"cloud.google.com/go/logging"
	"github.com/verbeux-ai/whatsmiau/env"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
)

func startGCL() (*zapcore.Core, error) {
	ctx := context.Background()
	client, err := logging.NewClient(ctx, env.Env.GCLProjectID)
	if err != nil {
		return nil, err
	}

	lg := client.Logger(env.Env.GCL)

	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	errorPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zap.ErrorLevel
	})
	warnPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zap.WarnLevel && lvl < zap.ErrorLevel
	})
	infoPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zap.InfoLevel && lvl < zap.WarnLevel
	})
	debugPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zap.DebugLevel && lvl < zap.InfoLevel
	})

	errorLogger := lg.StandardLogger(logging.Error)
	warnLogger := lg.StandardLogger(logging.Warning)
	infoLogger := lg.StandardLogger(logging.Info)
	debugLogger := lg.StandardLogger(logging.Debug)

	core := zapcore.NewTee(
		zapcore.NewCore(enc, zapcore.AddSync(errorLogger.Writer()), errorPriority),
		zapcore.NewCore(enc, zapcore.AddSync(warnLogger.Writer()), warnPriority),
		zapcore.NewCore(enc, zapcore.AddSync(infoLogger.Writer()), infoPriority),
		zapcore.NewCore(enc, zapcore.AddSync(debugLogger.Writer()), debugPriority),
	)

	zap.ReplaceGlobals(zap.New(core))

	return &core, nil
}
