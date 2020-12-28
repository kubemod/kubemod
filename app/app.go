package app

import (
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	kzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func iso8601TimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000Z0700"))
}

// NewLogger instantiates a new logger.
func NewLogger(enableDevModeLog EnableDevModeLog) logr.Logger {
	log := kzap.New(func(o *kzap.Options) {
		o.Development = bool(enableDevModeLog)

		// Enforce JSON encoder for both debug and non-debug deployments,
		// but ensure that timing is encoded using ISO8601.
		encCfg := zap.NewProductionEncoderConfig()
		encCfg.EncodeTime = iso8601TimeEncoder
		o.Encoder = zapcore.NewJSONEncoder(encCfg)
	})

	// Wire up the controller with the log.
	ctrl.SetLogger(log)

	return log
}
