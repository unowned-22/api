package logger

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/unowned-22/api/internal/contextx"
)

// FromContext возвращает logrus.Entry с request_id и user_id из контекста.
// Используется везде вместо прямого logger.Log.
func FromContext(ctx context.Context) *logrus.Entry {
	if Log == nil {
		return logrus.NewEntry(logrus.New())
	}

	entry := Log.WithFields(logrus.Fields{})

	if reqID, ok := contextx.GetRequestID(ctx); ok && reqID != "" {
		entry = entry.WithField("request_id", reqID)
	}
	if userID, ok := contextx.UserID(ctx); ok {
		entry = entry.WithField("user_id", userID)
	}
	return entry
}
