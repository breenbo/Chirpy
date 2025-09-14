package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	auth "github.com/breenbo/chirpy/internal"
	"github.com/breenbo/chirpy/internal/database"
	"github.com/breenbo/chirpy/internal/models"
)

type UserHandler struct {
	dbQueries *database.Queries
}

func NewUserHandler(dbQueries *database.Queries) *UserHandler {
	return &UserHandler{
		dbQueries: dbQueries,
	}
}

func (uh *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// get the body of the request
	decoder := json.NewDecoder(r.Body)
	req_body := models.CreateUserRequest{}
	err := decoder.Decode(&req_body)
	if err != nil {
		msg := fmt.Sprintf("Error parsing request body: %v", err)
		ReturnParseError(w, msg)
		return
	}

	// create user in db
	hashPass, err := auth.HashPassword(req_body.Password)
	if err != nil {
		msg := fmt.Sprintf("Error hashing password: %v", err)
		ReturnParseError(w, msg)
		return
	}
	res, err := uh.dbQueries.CreateUser(r.Context(), database.CreateUserParams{
		Email:          req_body.Email,
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
	// get the body of the request
	decoder := json.NewDecoder(r.Body)
	req_body := models.LoginRequest{}
	err := decoder.Decode(&req_body)
	if err != nil {
		msg := fmt.Sprintf("Error parsing request body: %v", err)
		ReturnParseError(w, msg)
		return
	}

	w.Header().Set("Content-type", "application/json;charset=utf-8")

	user, err := uh.dbQueries.GetUser(r.Context(), req_body.Email)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte("User not found"))
		return
	}

	passErr := auth.CheckPasswordHash(req_body.Password, user.HashedPassword)
	if passErr != nil {
		w.WriteHeader(401)
		w.Write([]byte("incorrect email or password"))
		return
	}

	w.WriteHeader(200)
	response := models.User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}
	data, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte("Error parsing response json"))
	} else {
		w.Write(data)
	}
}
