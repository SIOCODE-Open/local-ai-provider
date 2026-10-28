package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func GET_v1_models(w http.ResponseWriter, r *http.Request) {
	AppContext.mu.RLock()
	models := make([]ModelObject, len(AppContext.Models))
	copy(models, AppContext.Models)
	AppContext.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   models,
	})
}

func GET_v1_models_model(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	modelId := vars["model"]

	AppContext.mu.RLock()
	var found *ModelObject
	for i := range AppContext.Models {
		if AppContext.Models[i].Id == modelId {
			m := AppContext.Models[i]
			found = &m
			break
		}
	}
	AppContext.mu.RUnlock()

	if found == nil {
		writeOpenAIError(
			w,
			http.StatusNotFound,
			fmt.Sprintf("The model '%s' does not exist", modelId),
			"invalid_request_error",
			"model",
			"model_not_found",
		)
		return
	}

	writeJSON(w, http.StatusOK, found)
}
