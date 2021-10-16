package endpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
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

	SiGameArchivesPath = "/siq_archives"

	ToArchiveType = ".zip"
)

type GameType string

const (
	SiGame GameType = "si_game_pack"
	MyGame GameType = "my_game_pack"
)

func (g GameType) ToString() string {
	return string(g)
}

type EndpointType string

const (
	HubEndpoint             EndpointType = "/hub"
	AuthCredentialsEndpoint EndpointType = "/auth/credentials"
	AuthAccessEndpoint      EndpointType = "/auth/access"
	AuthGuest               EndpointType = "/auth/guest"
	GetLoginEndpoint        EndpointType = "/get/login/"
	RegisterEndpoint        EndpointType = "/register"
	PackUploadEndpoint      EndpointType = "/pack/upload"
	GetPacksEndpoint        EndpointType = "/get/packs"
	GetPackInfoEndpoint     EndpointType = "/get/pack/info"
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
	EndpointContext     = "ENDPOINT"
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
	http.HandleFunc(GetPacksEndpoint.ToString(), e.getPacks)
	http.HandleFunc(GetPackInfoEndpoint.ToString(), e.getPackInfo)
}

func (e *Endpoint) CreateContext(r *http.Request) context.Context {
	requestToken := r.Header.Get(RequestTokenHeader)

	endpointName := EndpointType(r.URL.RequestURI()).ToString()

	logger := e.logger.With(
		zap.String("endpoint", endpointName),
		zap.String("request_token", requestToken),
	)

	var ctx = context.WithValue(r.Context(), RequestTokenContext, requestToken)
	ctx = context.WithValue(r.Context(), LoggerContext, logger)
	ctx = context.WithValue(r.Context(), EndpointContext, endpointName)

	err := e.monitoring.IncGauge(&monitoring.Metric{
		Namespace: "http",
		Name:      "request_per_second",
		ConstLabels: map[string]string{
			"endpoint_name": endpointName,
			"is_server":     fmt.Sprintf("%t", true),
		},
	})
	if err != nil {
		logger.Error(
			"monitoring endpoint error",
			zap.Error(err),
		)
	}

	return ctx
}

func (e *Endpoint) pushMetrics(isServer bool, endpointName string, f func() error) (executionTime float64, err error) {
	executionTime, err = e.monitoring.ExecutionTime(&monitoring.Metric{
		Namespace: "http",
		Name:      "execution_time",
		ConstLabels: map[string]string{
			"endpoint_name": endpointName,
			"is_server":     fmt.Sprintf("%t", isServer),
		},
	}, f)

	if err != nil {
		_ = e.monitoring.Inc(
			&monitoring.Metric{
				Namespace: "http",
				Name:      endpointName,
			},
		)
	}

	return
}

func (e *Endpoint) getPacks(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		e.responseWriterError(errors.New("method not allowed"), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	type request struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}

	var req *request

	err = json.Unmarshal(body, &req)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	packs := singleton.GetPacks()

	type pack struct {
		Name string   `json:"name"`
		Hash [32]byte `json:"hash"`
	}

	packsResponse := make([]*pack, 0, len(packs))

	for hash, name := range packs {
		packsResponse = append(packsResponse, &pack{
			Name: name,
			Hash: hash,
		})
	}

	if req.Offset != 0 || req.Limit != 0 {
		if len(packsResponse) <= req.Offset {
			e.responseWriter(http.StatusOK, map[string]interface{}{
				"packs": "",
			}, w, ctx)

			return
		}

		if req.Offset+req.Limit > len(packsResponse) {
			req.Limit = len(packsResponse)
		}

		packsResponse = packsResponse[req.Offset : req.Offset+req.Limit]
	}

	e.responseWriter(http.StatusOK, map[string]interface{}{
		"packs": packsResponse,
	}, w, ctx)
}

func (e *Endpoint) getPackInfo(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		e.responseWriterError(errors.New("method not allowed"), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	type request struct {
		Hash [32]byte `json:"hash"`
	}

	var req *request

	err = json.Unmarshal(body, &req)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	packName := singleton.GetPack(req.Hash)

	err = helpers.Unzip(e.configuration.Pack.Path+SiGameArchivesPath+"/"+packName,
		e.configuration.PackTemporary.Path+"/"+packName)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "Unzip pack error")

		return
	}

	parser := NewParser(e.configuration.PackTemporary.Path)

	err = parser.ParsingSiGamePack(packName)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "parsing si game pack error")

		return
	}

	err = parser.InitMyGame()
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "init my game error")

		return
	}

	pack := parser.GetMyGame()

	for _, round := range pack.Rounds {
		for _, theme := range round.Themes {
			for _, quest := range theme.Quests {
				quest.Answer = []*Object{}
			}
		}
	}

	e.responseWriter(http.StatusOK, map[string]interface{}{
		"pack_info": pack,
	}, w, ctx)
}

func (e *Endpoint) saveSiGamePack(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		e.responseWriterError(errors.New("method not allowed"), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	multipartFile, fileHeader, err := r.FormFile(SiGame.ToString())
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "get data from form file error")

		return
	}

	_, err = jwt.ParseJWT([]byte(e.configuration.JWT.SecretKey), r.Header.Get("Authorization"))
	if err != nil {
		e.responseWriterError(err, w, http.StatusUnauthorized, ctx, "parse jwt error")

		return
	}

	if fileHeader.Size > MaxPackSize {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "file size > 150 MB")

		return
	}

	buf := bytes.NewBuffer(nil)
	if _, err = io.Copy(buf, multipartFile); err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "io copy error")

		return
	}

	hash := sha256.Sum256(buf.Bytes())

	ok := singleton.IsExistPack(hash)
	if !ok {
		singleton.AddPack(hash, fileHeader.Filename)

		file, err := os.Create(e.configuration.Pack.Path + SiGameArchivesPath + "/" + fileHeader.Filename + ToArchiveType)
		if err != nil {
			e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "save file error")

			return
		}

		if _, err = io.Copy(file, buf); err != nil {
			e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "io copy error")

			return
		}
	}

	e.responseWriter(http.StatusOK, map[string]interface{}{}, w, ctx)

	return
}

func (e *Endpoint) authCredentials(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		e.responseWriterError(errors.New("method not allowed"), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	var credentials *models.Credentials

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &credentials)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	err = credentials.Validate()
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "validate credentials error")

		return
	}

	if !e.repository.UserRepository.IsExistByLogin(ctx, credentials.Login) {
		e.responseWriterError(err, w, http.StatusUnauthorized, ctx, "user does not exist")

		return
	}

	hashPassword, err := helpers.NewMD5Hash(credentials.Password)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "hash password error")

		return
	}

	credentials.Password = hashPassword

	id, err := e.repository.UserRepository.GetUserByCredentials(ctx, credentials)
	if err != nil {
		e.responseWriterError(err, w, http.StatusUnauthorized, ctx, "hash password error")

		return
	}

	token, err := jwt.GenerateTokens(ctx, id, credentials.Login, e.configuration.JWT.SecretKey,
		e.configuration.JWT.ExpirationTime)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "generate token error")

		return
	}

	e.responseWriter(http.StatusOK, map[string]interface{}{
		"access_token": token,
	}, w, ctx)

	return
}

func (e *Endpoint) authAccessToken(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		e.responseWriterError(errors.New("method not allowed"), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	type request struct {
		AccessToken string `json:"access_token"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	token, err := jwt.ParseJWT([]byte(e.configuration.JWT.SecretKey), req.AccessToken)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "parse jwt error")

		return
	}

	if token.ExpiresAt < time.Now().Unix() {
		e.responseWriterError(errors.New("token has expired"), w, http.StatusUnauthorized, ctx, "")

		return
	}

	e.responseWriter(http.StatusOK, map[string]interface{}{}, w, ctx)

	return
}

func (e *Endpoint) authGuest(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		e.responseWriterError(errors.New("method not allowed"), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	type request struct {
		Login string `json:"login"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	token, err := jwt.GenerateTokens(ctx, 0, req.Login, e.configuration.JWT.SecretKey, e.configuration.JWT.ExpirationTime)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "generate token error")

		return
	}

	e.responseWriter(http.StatusOK, map[string]interface{}{
		"access_token": token,
	}, w, ctx)

	return
}

func (e *Endpoint) getLoginFromAccessToken(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		e.responseWriterError(errors.New("method not allowed"), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	type request struct {
		AccessToken string `json:"access_token"`
	}

	var req *request

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &req)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	token, err := jwt.ParseJWT([]byte(e.configuration.JWT.SecretKey), req.AccessToken)
	if err != nil {
		e.responseWriterError(err, w, http.StatusUnauthorized, ctx, "parse jwt error")

		return
	}

	if token.ExpiresAt < time.Now().Unix() {
		e.responseWriterError(errors.New("token has expired"), w, http.StatusUnauthorized, ctx, "")

		return
	}

	e.responseWriter(http.StatusOK, map[string]interface{}{
		"login": token.Login,
	}, w, ctx)

	return
}

func (e *Endpoint) createUser(w http.ResponseWriter, r *http.Request) {
	ctx := e.CreateContext(r)

	if r.Method != http.MethodPost {
		e.responseWriterError(errors.New("method not allowed"), w, http.StatusMethodNotAllowed, ctx, "")

		return
	}

	var user *models.User

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "read body error")

		return
	}

	err = json.Unmarshal(body, &user)
	if err != nil {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "unmarshal body to struct error")

		return
	}

	if e.repository.UserRepository.IsExistByLogin(ctx, user.Login) {
		e.responseWriterError(err, w, http.StatusBadRequest, ctx, "user does not exist")

		return
	}

	hashPassword, err := helpers.NewMD5Hash(user.Password)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "hash password error")

		return
	}

	user.Password = hashPassword

	id, err := e.repository.UserRepository.CreateUser(ctx, user)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "create user error")

		return
	}

	token, err := jwt.GenerateTokens(ctx, id, user.Login, e.configuration.JWT.SecretKey, e.configuration.JWT.ExpirationTime)
	if err != nil {
		e.responseWriterError(err, w, http.StatusInternalServerError, ctx, "parse jwt error")

		return
	}

	e.responseWriter(http.StatusOK, map[string]interface{}{
		"access_token": token,
	}, w, ctx)

	return
}
