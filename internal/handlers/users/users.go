package users

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/breenbo/chirpy/internal/auth"
	"github.com/breenbo/chirpy/internal/database"
	"github.com/breenbo/chirpy/internal/handlers"
	"github.com/breenbo/chirpy/internal/models"
	"github.com/google/uuid"
)

type UserHandler struct {
	dbQueries *database.Queries
	jwtSecret string
}

func NewUserHandler(dbQueries *database.Queries, jwtSecret string) *UserHandler {
	return &UserHandler{
		dbQueries: dbQueries,
		jwtSecret: jwtSecret,
	}
}

func (uh *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// get the body of the request
	decoder := json.NewDecoder(r.Body)
	reqBody := models.CreateUserRequest{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		msg := fmt.Sprintf("Error parsing request body: %v", err)
		handlers.ReturnParseError(w, msg)
		return
	}

	// create user in db
	hashPass, err := auth.HashPassword(reqBody.Password)
	if err != nil {
		msg := fmt.Sprintf("Error hashing password: %v", err)
		handlers.ReturnParseError(w, msg)
		return
	}

	res, err := uh.dbQueries.CreateUser(r.Context(), database.CreateUserParams{
		Email:          reqBody.Email,
		HashedPassword: hashPass,
	})
	if err != nil {
		msg := fmt.Sprintf("Error creating user: %v", err)
		handlers.ReturnParseError(w, msg)
		return
	}

	// return the user after being created
	w.Header().Set("Content-type", "application/json;charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	response := models.User{
		ID:          res.ID,
		CreatedAt:   res.CreatedAt,
		UpdatedAt:   res.UpdatedAt,
		Email:       res.Email,
		IsChirpyRed: res.IsChirpyRed,
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
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
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

// UpgradeUser: upgrade user on webhook
type data struct {
	UserID uuid.UUID `json:"user_id"`
}
type upgradeWebhook struct {
	Event string `json:"event"`
	Data  data   `json:"data"`
}

const upgradeEvent = "user.upgraded"

func (uh *UserHandler) UpgradeUser(w http.ResponseWriter, r *http.Request) {
	// get the body of the webhook
	decoder := json.NewDecoder(r.Body)
	reqBody := upgradeWebhook{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// get the event and the user id from the body
	event := reqBody.Event

	// if event != user.upgrader => response with 204
	if event != upgradeEvent {
		w.WriteHeader(http.StatusNoContent)
		return
	} else {
		// if event = user.upgraded => set in DB
		err := uh.dbQueries.UpgradeUser(r.Context(), reqBody.Data.UserID)
		if err != nil {
			// if error 404 (user not found)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// if success => 204
		w.WriteHeader(http.StatusNoContent)
	}
}
