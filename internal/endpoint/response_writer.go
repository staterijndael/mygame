package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"mygame/dependers/monitoring"
	"net/http"
)

func (e *Endpoint) responseWriter(statusCode int, data interface{}, w http.ResponseWriter, ctx context.Context) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	json, err := json.Marshal(data)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "sent response error")

		return
	}

	_, err = w.Write(json)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "sent response error")

		return
	}

	e.monitoring.DecGauge(&monitoring.Metric{
		Namespace: "http",
		Name:      "request_per_second",
		ConstLabels: map[string]string{
			"endpoint_name": ctx.Value(EndpointContext).(string),
			"is_server":     fmt.Sprintf("%t", true),
		},
	})
}

func (e *Endpoint) responseWriterError(err error, w http.ResponseWriter, statusCode int, ctx context.Context, message string) {
	e.responseWriter(statusCode, map[string]interface{}{
		"error": err.Error(),
	}, w, ctx)

	return
}
