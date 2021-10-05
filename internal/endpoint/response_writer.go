package endpoint

import (
	"context"
	"encoding/json"
	"go.uber.org/zap"
	"net/http"
)

func responseWriter(statusCode int, data interface{}, w http.ResponseWriter, ctx context.Context) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	json, err := json.Marshal(data)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "sent response error")

		return
	}

	_, err = w.Write(json)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "sent response error")

		return
	}
}

func responseWriterError(err error, w http.ResponseWriter, statusCode int, ctx context.Context, message string) {
	logger := ctx.Value("LOGGER").(*zap.Logger)

	logger.Error(
		message,
		zap.Error(err),
	)

	responseWriter(statusCode, map[string]interface{}{
		"error": err.Error(),
	}, w, ctx)

	return
}
