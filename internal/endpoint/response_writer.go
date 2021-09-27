package endpoint

import (
	"encoding/json"
	"net/http"
)

func responseWriter(statusCode int, data interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	json, err := json.Marshal(data)
	if err != nil {
		responseWriter(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		}, w)

		return
	}

	_, err = w.Write(json)
	if err != nil {
		responseWriter(http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		}, w)

		return
	}
}

func responseWriterError(err error, w http.ResponseWriter, statusCode int) {
	responseWriter(statusCode, map[string]interface{}{
		"error": err.Error(),
	}, w)

	return
}
