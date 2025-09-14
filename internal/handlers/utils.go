package handlers

import (
	"encoding/json"
	"net/http"
)

func ReturnParseError(w http.ResponseWriter, msg string) {
	type errorRes struct {
		Error string `json:"error"`
	}
	w.Header().Set("Content-type", "application/json;charset=utf-8")
	w.WriteHeader(500)
	resBody := errorRes{
		Error: msg,
	}
	data, err := json.Marshal(resBody)
	if err != nil {
		w.Write([]byte("error parsing json"))
	} else {
		w.Write(data)
	}
}
