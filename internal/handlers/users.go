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
	w.WriteHeader(http.StatusCreated)
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

func (uh *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	// check if the accesstoken is valid
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Error getting token in header"))
		return
	}

	userID, err := auth.ValidateJWT(token, uh.jwtSecret)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Error validating token"))
		return
	}

	// get the body data
	decoder := json.NewDecoder(r.Body)
	reqbody := models.CreateUserRequest{}
	err = decoder.Decode(&reqbody)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error parsing the request body"))
	}
	hashedPassword, err := auth.HashPassword(reqbody.Password)
	if err != nil {
		{
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error hashing password"))
		}
	}

	// update the user in the db
	user, err := uh.dbQueries.UpdateUser(r.Context(), database.UpdateUserParams{
		ID:             userID,
		Email:          reqbody.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error updating user"))
		return
	}

	response := models.User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}
	data, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error parsing response json"))
	} else {
		w.Header().Set("Content-type", "application/json;charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

func (uh *UserHandler) ResetUsers(w http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")
	if platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err := uh.dbQueries.ResetUsers(r.Context())
	if err != nil {
		w.Header().Add("Content-type", "application/json;charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
}

func (uh *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	// get the body of the request
	decoder := json.NewDecoder(r.Body)
	reqBody := models.LoginRequest{}

	err := decoder.Decode(&reqBody)
	if err != nil {
		msg := fmt.Sprintf("Error parsing request body: %v", err)
		ReturnParseError(w, msg)
		return
	}

	w.Header().Set("Content-type", "application/json;charset=utf-8")

	user, err := uh.dbQueries.GetUser(r.Context(), reqBody.Email)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("User not found"))
		return
	}

	match, err := auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
		return
	}
	if !match {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("incorrect email or password"))
		return
	}

	// create a accessToken for the logged in user
	accessExpirationTime := time.Hour
	accessToken, err := auth.MakeJWT(user.ID, uh.jwtSecret, time.Duration(accessExpirationTime))
	if err != nil {
		w.Write([]byte("Error creating token"))
		return
	}
	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		w.Write([]byte("Error creating refreshtoken"))
		return
	}

	_, err = uh.dbQueries.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().UTC().Add(time.Hour * 24 * 60),
	})
	if err != nil {
		w.Write([]byte("Error creating refreshtoken"))
		return
	}

	w.WriteHeader(http.StatusOK)
	type response struct {
		models.User
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}

	data, err := json.Marshal(response{
		User: models.User{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email:     user.Email,
		},
		Token:        accessToken,
		RefreshToken: refreshToken,
	})
	if err != nil {
		w.Write([]byte("Error parsing response json"))
	} else {
		w.Write(data)
	}
}

func (uh *UserHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error getting token in header"))
		return
	}

	user, err := uh.dbQueries.GetUserFromRefreshToken(r.Context(), refreshToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Error getting user"))
		return
	}

	accessToken, err := auth.MakeJWT(user.ID, uh.jwtSecret, time.Hour)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Error getting accessToken"))
		return
	}

	type response struct {
		Token string `json:"token"`
	}
	data, err := json.Marshal(response{
		Token: accessToken,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Unable to serialize token"))
		return
	} else {
		w.Write(data)
	}
}

func (uh *UserHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error getting token in header"))
		return
	}

	_, err = uh.dbQueries.RevokeRefreshToken(r.Context(), refreshToken)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Error revoking token"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
