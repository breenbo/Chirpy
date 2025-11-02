package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/breenbo/chirpy/internal/auth"
	"github.com/breenbo/chirpy/internal/database"
	"github.com/breenbo/chirpy/internal/models"
)

type UserHandler struct {
	dbQueries *database.Queries
	jwtSecret string
}

func NewUserHandler(dbQueries *database.Queries, jwtSecret string) *UserHandler {
	return &UserHandler{
		dbQueries: dbQueries,
		jwtSecret: jwtSecret, // access to secret from apiCfg
	}
}

func (uh *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// get the body of the request
	decoder := json.NewDecoder(r.Body)
	reqBody := models.CreateUserRequest{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		msg := fmt.Sprintf("Error parsing request body: %v", err)
		ReturnParseError(w, msg)
		return
	}

	// create user in db
	hashPass, err := auth.HashPassword(reqBody.Password)
	if err != nil {
		msg := fmt.Sprintf("Error hashing password: %v", err)
		ReturnParseError(w, msg)
		return
	}
	res, err := uh.dbQueries.CreateUser(r.Context(), database.CreateUserParams{
		Email:          reqBody.Email,
		HashedPassword: hashPass,
	})
	if err != nil {
		msg := fmt.Sprintf("Error creating user: %v", err)
		ReturnParseError(w, msg)
		return
	}

	// return the user after being created
	w.Header().Set("Content-type", "application/json;charset=utf-8")
	w.WriteHeader(201)
	response := models.User{
		ID:        res.ID,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
		Email:     res.Email,
	}
	data, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte("Error parsing response json"))
	} else {
		w.Write(data)
	}
}

func (uh *UserHandler) ResetUsers(w http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")
	if platform != "dev" {
		w.WriteHeader(403)
		return
	}

	err := uh.dbQueries.ResetUsers(r.Context())
	if err != nil {
		w.Header().Add("Content-type", "application/json;charset=utf-8")
		w.WriteHeader(500)
	}

	w.WriteHeader(200)
}

func (uh *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	const defaultExpiration = 3600

	// get the body of the request
	decoder := json.NewDecoder(r.Body)
	reqBody := models.LoginRequest{}

	err := decoder.Decode(&reqBody)
	if err != nil {
		msg := fmt.Sprintf("Error parsing request body: %v", err)
		ReturnParseError(w, msg)
		return
	}

	// manage expiration time, set to 1 hour if not present
	expirationTime := defaultExpiration
	if reqBody.ExpiresInSeconds != nil {
		expirationTime = *reqBody.ExpiresInSeconds
		if *reqBody.ExpiresInSeconds > 3600 {
			expirationTime = defaultExpiration
		}
	}

	w.Header().Set("Content-type", "application/json;charset=utf-8")

	user, err := uh.dbQueries.GetUser(r.Context(), reqBody.Email)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte("User not found"))
		return
	}

	passErr, err := auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)
	if err != nil {
		return
	}
	if passErr {
		w.WriteHeader(401)
		w.Write([]byte("incorrect email or password"))
		return
	}

	// create a token for the logged in user
	token, err := auth.MakeJWT(user.ID, uh.jwtSecret, time.Duration(expirationTime))
	if err != nil {
		return
	}

	w.WriteHeader(200)
	response := models.User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
		Token:     token,
	}
	data, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte("Error parsing response json"))
	} else {
		w.Write(data)
	}
}
