package endpoint

import (
	"encoding/json"
	"errors"
	"github.com/jmoiron/sqlx"
	"io/ioutil"
	"mygame/config"
	"mygame/internal/models"
	"mygame/internal/repository"
	"mygame/tools/helpers"
	"mygame/tools/jwt"
	"net/http"
	"time"
)

type Endpoint struct {
	repository    *repository.Repository
	configuration *config.Config
}

func NewEndpoint(db *sqlx.DB, config *config.Config) *Endpoint {
	return &Endpoint{
		repository:    repository.NewRepository(db),
		configuration: config,
	}
}

func (e *Endpoint) InitRoutes() {
	http.HandleFunc("/auth/credentials", e.authCredentials)
	http.HandleFunc("/auth/access", e.authAccessToken)
	http.HandleFunc("/auth/refresh", e.refreshTokens)
	http.HandleFunc("/auth/guest", e.authGuest)
	http.HandleFunc("/get/login", e.getLoginFromAccessToken)
	http.HandleFunc("/register", e.createUser)
	http.HandleFunc("/hub", e.serveWs)
}

func (e *Endpoint) authCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseWriter(http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed",
		}, w)

		return
	}

	var credentials *models.Credentials

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = json.Unmarshal(body, &credentials)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	err = credentials.Validate()
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	if !e.repository.UserRepository.IsExistByLogin(r.Context(), credentials.Login) {
		responseWriterError(err, w, http.StatusUnauthorized)

		return
	}

	hashPassword, err := helpers.NewMD5Hash(credentials.Password)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	credentials.Password = hashPassword

	id, err := e.repository.UserRepository.GetUserByCredentials(r.Context(), credentials)
	if err != nil {
		responseWriterError(err, w, http.StatusUnauthorized)

		return
	}

	tokens, err := jwt.GenerateTokens(r.Context(), id, credentials.Login, e.configuration.JWT.SecretKey, e.configuration.JWT.ExpirationTime)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = e.repository.TokenRepository.CreateToken(r.Context(), tokens)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
	}, w)

	return
}

func (e *Endpoint) authAccessToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseWriter(http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed",
		}, w)

		return
	}

	type request struct {
		AccessToken string `json:"access_token"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	token, err := jwt.ParseJWT([]byte(e.configuration.JWT.SecretKey), req.AccessToken)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	if token.ExpiresAt < time.Now().Unix() {
		responseWriterError(errors.New("token has expired"), w, http.StatusUnauthorized)

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{}, w)

	return
}

func (e *Endpoint) refreshTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseWriter(http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed",
		}, w)

		return
	}

	type request struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	tokens, err := e.repository.TokenRepository.GetTokenByAccessToken(r.Context(), req.AccessToken)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	if !(tokens.RefreshToken == req.RefreshToken && tokens.AccessToken == req.AccessToken) {
		responseWriterError(errors.New("invalid tokens"), w, http.StatusUnauthorized)

		return
	}

	tokensNew, err := jwt.GenerateTokens(r.Context(), tokens.ID, tokens.Login, e.configuration.JWT.SecretKey, e.configuration.JWT.ExpirationTime)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = e.repository.TokenRepository.CreateToken(r.Context(), tokens)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{
		"access_token":  tokensNew.AccessToken,
		"refresh_token": tokensNew.RefreshToken,
	}, w)

	return
}

func (e *Endpoint) authGuest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseWriter(http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed",
		}, w)

		return
	}

	type request struct {
		Login string `json:"login"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	tokens, err := jwt.GenerateTokens(r.Context(), 0, req.Login, e.configuration.JWT.SecretKey, e.configuration.JWT.ExpirationTime)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = e.repository.TokenRepository.CreateToken(r.Context(), tokens)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
	}, w)

	return
}

func (e *Endpoint) getLoginFromAccessToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseWriter(http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed",
		}, w)

		return
	}

	type request struct {
		AccessToken string `json:"access_token"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	token, err := jwt.ParseJWT([]byte(e.configuration.JWT.SecretKey), req.AccessToken)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	if token.ExpiresAt < time.Now().Unix() {
		responseWriterError(errors.New("token has expired"), w, http.StatusUnauthorized)

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{
		"login": token.Login,
	}, w)

	return
}

func (e *Endpoint) createUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseWriter(http.StatusMethodNotAllowed, map[string]interface{}{
			"error": "method not allowed",
		}, w)

		return
	}

	var user *models.User

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = json.Unmarshal(body, &user)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	if e.repository.UserRepository.IsExistByLogin(r.Context(), user.Login) {
		responseWriterError(err, w, http.StatusBadRequest)

		return
	}

	hashPassword, err := helpers.NewMD5Hash(user.Password)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	user.Password = hashPassword

	id, err := e.repository.UserRepository.CreateUser(r.Context(), user)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	tokens, err := jwt.GenerateTokens(r.Context(), id, user.Login, e.configuration.JWT.SecretKey, e.configuration.JWT.ExpirationTime)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	err = e.repository.TokenRepository.CreateToken(r.Context(), tokens)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError)

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
	}, w)

	return
}
