package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/breenbo/chirpy/internal/auth"
	"github.com/breenbo/chirpy/internal/database"
	"github.com/breenbo/chirpy/internal/models"
	"github.com/google/uuid"
)

func replaceBadWords(body string) string {
	badwords := []string{"kerfuffle", "sharbert", "fornax"}
	strArray := strings.Split(body, " ")

	for i, word := range strArray {
		if slices.Contains(badwords, strings.ToLower(word)) {
			strArray[i] = "****"
		}
	}

	return strings.Join(strArray, " ")
}

type ChirpHandler struct {
	dbQueries *database.Queries
	jwtSecret string
}

func NewChirpHandler(dbQueries *database.Queries, jwtSecret string) *ChirpHandler {
	return &ChirpHandler{
		dbQueries: dbQueries,
		jwtSecret: jwtSecret, // access to secret from apiCfg
	}
}

func (ch *ChirpHandler) CreateChirps(w http.ResponseWriter, r *http.Request) {
	type Request struct {
		Body string `json:"body"`
	}
	type errorRes struct {
		Error string `json:"error"`
	}

	// check the token is valid
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		msg := fmt.Sprintf("Error getting token: %v", err)
		ReturnParseError(w, msg)
		return
	}
	userID, err := auth.ValidateJWT(token, ch.jwtSecret)
	if err != nil {
		msg := fmt.Sprintf("Error validating token: %v", err)
		ReturnParseError(w, msg)
		return
	}

	// get the body of the request
	decoder := json.NewDecoder(r.Body)
	reqBody := Request{}
	err = decoder.Decode(&reqBody)
	if err != nil {
		msg := fmt.Sprintf("Error parsing request body: %v", err)
		ReturnParseError(w, msg)
		return
	}

	// validate the chirp len
	if len(reqBody.Body) > 140 {
		w.Header().Set("Content-type", "application/json;charset=utf-8")
		w.WriteHeader(400)
		resBody := errorRes{
			Error: "Chirp is too long",
		}
		data, err := json.Marshal(resBody)
		if err != nil {
			w.Write([]byte("error parsing json"))
		} else {
			w.Write(data)
		}
		return
	}

	// create chirp on db
	res, err := ch.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   replaceBadWords(reqBody.Body),
		UserID: userID,
	})
	if err != nil {
		msg := fmt.Sprintf("Error creating chirp: %v", err)
		ReturnParseError(w, msg)
		return
	}

	// return the created created chirp
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(201)
	response := models.Chirp{
		ID:        res.ID,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
		Body:      res.Body,
		UserID:    res.UserID.String(),
	}
	data, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte("Error parsing response json"))
	} else {
		w.Write(data)
	}
}

func (ch *ChirpHandler) GetChirps(w http.ResponseWriter, r *http.Request) {
	// get chirps from db
	res, err := ch.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		msg := fmt.Sprintf("Error gettings chirps: %v", err)
		ReturnParseError(w, msg)
		return
	}
	// return the chirps
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(200)
	response := []models.Chirp{}
	for _, chirp := range res {
		response = append(response, models.Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID.String(),
		})
	}

	data, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte("Error parsing response json"))
	} else {
		w.Write(data)
	}
}

func (ch *ChirpHandler) GetOneChirp(w http.ResponseWriter, r *http.Request) {
	chirpIDStr := r.PathValue("id")
	if chirpIDStr == "" {
		msg := "missing chirp id"
		ReturnParseError(w, msg)
		return
	}
	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		msg := fmt.Sprintf("Error parsing chirp id: %v", err)
		ReturnParseError(w, msg)
		return
	}

	res, err := ch.dbQueries.GetOneChirp(r.Context(), chirpID)
	if err != nil {
		if err == sql.ErrNoRows {
			msg := fmt.Sprintf("Error getting chirp %s: %v", chirpID, err)
			w.Header().Set("Content-Type", "application/json;charset=utf-8")
			w.WriteHeader(404)
			w.Write([]byte(msg))
			return
		}
	}

	// return the chirps
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(200)
	response := models.Chirp{
		ID:        res.ID,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
		Body:      res.Body,
		UserID:    res.UserID.String(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte("Error parsing response json"))
	} else {
		w.Write(data)
	}
}
