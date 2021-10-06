package endpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"mygame/config"
	"mygame/dependers/monitoring"
	"mygame/internal/models"
	"mygame/internal/repository"
	"mygame/internal/singleton"
	"mygame/tools/helpers"
	"mygame/tools/jwt"
	"net/http"
	"os"
	"time"
)

const (
	MB = 1 << 20

	MaxPackSize = MB * 150

	SiGame = "si_game_pack"
	MyGame = "my_game_pack"

	SiGameArchivesPath = "/siq_archives"

	ToArchiveType = ".zip"
)

type EndpointType string

const (
	HubEndpoint             EndpointType = "/hub"
	AuthCredentialsEndpoint EndpointType = "/auth/credentials"
	AuthAccessEndpoint      EndpointType = "/auth/access"
	AuthGuest               EndpointType = "/auth/guest"
	GetLoginEndpoint        EndpointType = "/get/login/"
	RegisterEndpoint        EndpointType = "/register"
	PackUploadEndpoint      EndpointType = "/pack/upload"
)

func (e EndpointType) ToString() string {
	return string(e)
}

const (
	RequestTokenHeader = "X-REQUEST-TOKEN"
)

const (
	RequestTokenContext = "REQUEST_TOKEN"
	LoggerContext       = "LOGGER"
)

type Endpoint struct {
	repository    *repository.Repository
	configuration *config.Config
	logger        *zap.Logger
	monitoring    monitoring.IMonitoring
}

func NewEndpoint(db *sqlx.DB, config *config.Config, logger *zap.Logger, monitoring monitoring.IMonitoring) *Endpoint {
	return &Endpoint{
		repository:    repository.NewRepository(db),
		configuration: config,
		logger:        logger,
		monitoring:    monitoring,
	}
}

func (e *Endpoint) InitRoutes() {
	http.HandleFunc(AuthCredentialsEndpoint.ToString(), e.authCredentials)
	http.HandleFunc(AuthAccessEndpoint.ToString(), e.authAccessToken)
	http.HandleFunc(AuthGuest.ToString(), e.authGuest)
	http.HandleFunc(GetLoginEndpoint.ToString(), e.getLoginFromAccessToken)
	http.HandleFunc(RegisterEndpoint.ToString(), e.createUser)
	http.HandleFunc(HubEndpoint.ToString(), e.serveWs)
	http.HandleFunc(PackUploadEndpoint.ToString(), e.saveSiGamePack)
}

func (e *Endpoint) CreateContext(r *http.Request) context.Context {
	requestToken := r.Header.Get(RequestTokenHeader)

	logger := e.logger.With(
		zap.String("endpoint", EndpointType(r.URL.RequestURI()).ToString()),
		zap.String("request_token", requestToken),
	)

	ctx := context.WithValue(r.Context(), RequestTokenContext, requestToken)
	ctx = context.WithValue(r.Context(), LoggerContext, logger)

	return ctx
}

func (e *Endpoint) saveSiGamePack(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		responseWriterError(errors.New("method not allowed").(error), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	multipartFile, fileHeader, err := r.FormFile(SiGame)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "get data from form file error")

		return
	}

	_, err = jwt.ParseJWT([]byte(e.configuration.JWT.SecretKey), r.Header.Get("Authorization"))
	if err != nil {
		responseWriterError(err, w, http.StatusUnauthorized, ctx, "parse jwt error")

		return
	}

	if fileHeader.Size > MaxPackSize {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "file size > 150 MB")

		return
	}

	buf := bytes.NewBuffer(nil)
	if _, err = io.Copy(buf, multipartFile); err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "io copy error")

		return
	}

	hash := sha256.Sum256(buf.Bytes())

	ok := singleton.IsExistPack(hash)
	if !ok {
		singleton.AddPack(hash, fileHeader.Filename)

		file, err := os.Create(e.configuration.Pack.Path + SiGameArchivesPath + "/" + fileHeader.Filename + ToArchiveType)
		if err != nil {
			responseWriterError(err, w, http.StatusInternalServerError, ctx, "save file error")

			return
		}

		if _, err = io.Copy(file, buf); err != nil {
			responseWriterError(err, w, http.StatusInternalServerError, ctx, "io copy error")

			return
		}
	}

	responseWriter(http.StatusOK, map[string]interface{}{}, w, ctx)

	return
}

func (e *Endpoint) authCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		responseWriterError(errors.New("method not allowed").(error), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	var credentials *models.Credentials

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &credentials)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	err = credentials.Validate()
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "validate credentials error")

		return
	}

	if !e.repository.UserRepository.IsExistByLogin(r.Context(), credentials.Login) {
		responseWriterError(err, w, http.StatusUnauthorized, ctx, "user does not exist")

		return
	}

	hashPassword, err := helpers.NewMD5Hash(credentials.Password)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "hash password error")

		return
	}

	credentials.Password = hashPassword

	id, err := e.repository.UserRepository.GetUserByCredentials(r.Context(), credentials)
	if err != nil {
		responseWriterError(err, w, http.StatusUnauthorized, ctx, "hash password error")

		return
	}

	token, err := jwt.GenerateTokens(r.Context(), id, credentials.Login, e.configuration.JWT.SecretKey,
		e.configuration.JWT.ExpirationTime)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "generate token error")

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{
		"access_token": token,
	}, w, ctx)

	return
}

func (e *Endpoint) authAccessToken(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		responseWriterError(errors.New("method not allowed").(error), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	type request struct {
		AccessToken string `json:"access_token"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	token, err := jwt.ParseJWT([]byte(e.configuration.JWT.SecretKey), req.AccessToken)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "parse jwt error")

		return
	}

	if token.ExpiresAt < time.Now().Unix() {
		responseWriterError(errors.New("token has expired").(error), w, http.StatusUnauthorized, ctx, "")

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{}, w, ctx)

	return
}

func (e *Endpoint) authGuest(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		responseWriterError(errors.New("method not allowed").(error), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	type request struct {
		Login string `json:"login"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	token, err := jwt.GenerateTokens(r.Context(), 0, req.Login, e.configuration.JWT.SecretKey, e.configuration.JWT.ExpirationTime)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "generate token error")

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{
		"access_token": token,
	}, w, ctx)

	return
}

func (e *Endpoint) getLoginFromAccessToken(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		responseWriterError(errors.New("method not allowed").(error), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	type request struct {
		AccessToken string `json:"access_token"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	token, err := jwt.ParseJWT([]byte(e.configuration.JWT.SecretKey), req.AccessToken)
	if err != nil {
		responseWriterError(err, w, http.StatusUnauthorized, ctx, "parse jwt error")

		return
	}

	if token.ExpiresAt < time.Now().Unix() {
		responseWriterError(errors.New("token has expired").(error), w, http.StatusUnauthorized, ctx, "")

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{
		"login": token.Login,
	}, w, ctx)

	return
}

func (e *Endpoint) createUser(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		responseWriterError(errors.New("method not allowed").(error), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	var user *models.User

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &user)
	if err != nil {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	if e.repository.UserRepository.IsExistByLogin(r.Context(), user.Login) {
		responseWriterError(err, w, http.StatusBadRequest, ctx, "user does not exist")

		return
	}

	hashPassword, err := helpers.NewMD5Hash(user.Password)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "hash password error")

		return
	}

	user.Password = hashPassword

	id, err := e.repository.UserRepository.CreateUser(r.Context(), user)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "create user error")

		return
	}

	token, err := jwt.GenerateTokens(r.Context(), id, user.Login, e.configuration.JWT.SecretKey, e.configuration.JWT.ExpirationTime)
	if err != nil {
		responseWriterError(err, w, http.StatusInternalServerError, ctx, "parse jwt error")

		return
	}

	responseWriter(http.StatusOK, map[string]interface{}{
		"access_token": token,
	}, w, ctx)

	return
}
