package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
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
	res := []database.Chirp{}
	authorID, err := uuid.Parse(r.URL.Query().Get("author_id"))
	if err != nil {
		res, err = ch.dbQueries.GetAllChirps(r.Context())
	} else {
		res, err = ch.dbQueries.GetUserChirps(r.Context(), authorID)
	}

	// get chirps from db
	if err != nil {
		msg := fmt.Sprintf("Error gettings chirps: %v", err)
		ReturnParseError(w, msg)
		return
	}
	// return the chirps
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(200)
	chirps := []models.Chirp{}
	for _, chirp := range res {
		chirps = append(chirps, models.Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID.String(),
		})
	}

	// sort chirps depending on sort query param
	sortDirection := "asc"
	sortDirectionParam := r.URL.Query().Get("sort")
	if sortDirectionParam == "desc" {
		sortDirection = "desc"
	}

	sort.Slice(chirps, func(i, j int) bool {
		if sortDirection == "desc" {
			return chirps[i].CreatedAt.After(chirps[j].CreatedAt)
		}
		return chirps[i].CreatedAt.Before(chirps[j].CreatedAt)
	})

	data, err := json.Marshal(chirps)
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
		return
	} else {
		w.Write(data)
	}
}

func (ch *ChirpHandler) DeleteChirp(w http.ResponseWriter, r *http.Request) {
	// 1. check the access token in Authorization
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Missing access token"))
		return
	}
	userID, err := auth.ValidateJWT(token, ch.jwtSecret)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Error validating token"))
		return
	}
	// 2. delete chirp only if author logged in (-> 403 if not)
	chirpIDStr := r.PathValue("id")
	if chirpIDStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing chirp uuid"))
		return
	}
	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing chirp uuid"))
		return
	}
	chirp, err := ch.dbQueries.GetOneChirp(r.Context(), chirpID)
	// 4. id chirps not found -> 404
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if userID != chirp.UserID {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("You are not the author of this chirp"))
		return
	}

	// 3. delete the chirps from the db (-> 204)
	err = ch.dbQueries.DeleteChirp(r.Context(), chirpID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("unable to delete chirp"))
	}

	w.WriteHeader(http.StatusNoContent)
}
