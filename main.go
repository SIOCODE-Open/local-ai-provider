package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.LUTC)
	InitialConfigFile = LoadConfig()
	AppContext = NewAppContext(InitialConfigFile)
	port := AppContext.Config.Port

	log.Printf("Starting local-ai-provider on port %d", port)
	log.Printf("Number of configured models: %d", len(AppContext.Models))
	log.Printf("Number of configured matchers: %d", len(AppContext.Matchers))

	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/healthz", GET_healthz).Methods("GET")

	// OpenAI Models API
	router.HandleFunc("/v1/models", GET_v1_models).Methods("GET")
	router.HandleFunc("/v1/models/{model}", GET_v1_models_model).Methods("GET")

	// OpenAI Chat Completions API
	router.HandleFunc("/v1/chat/completions", POST_v1_chat_completions).Methods("POST")

	// OpenAI Responses API
	router.HandleFunc("/v1/responses", POST_v1_responses).Methods("POST")

	// Vertex AI / Gemini GenerateContent & StreamGenerateContent APIs
	router.HandleFunc("/v1beta/models/{model_action:.+}", POST_vertex_model_action).Methods("POST")
	router.HandleFunc("/v1/projects/{project}/locations/{location}/publishers/google/models/{model_action:.+}", POST_vertex_model_action).Methods("POST")
	router.HandleFunc("/v1beta/projects/{project}/locations/{location}/publishers/google/models/{model_action:.+}", POST_vertex_model_action).Methods("POST")

	// Testing, Inspection & Runtime Management API
	router.HandleFunc("/testing/requests", GET_testing_requests).Methods("GET")
	router.HandleFunc("/testing/requests/{id}", GET_testing_requests_id).Methods("GET")
	router.HandleFunc("/testing/requests", DELETE_testing_requests).Methods("DELETE")

	router.HandleFunc("/testing/matchers", GET_testing_matchers).Methods("GET")
	router.HandleFunc("/testing/matchers/{id}", GET_testing_matchers_id).Methods("GET")
	router.HandleFunc("/testing/matchers/{id}", PUT_testing_matchers_id).Methods("PUT")
	router.HandleFunc("/testing/matchers/{id}", DELETE_testing_matchers_id).Methods("DELETE")

	router.HandleFunc("/testing/models/{id}", PUT_testing_models_id).Methods("PUT")
	router.HandleFunc("/testing/models/{id}", DELETE_testing_models_id).Methods("DELETE")

	router.HandleFunc("/testing/config", GET_testing_config).Methods("GET")
	router.HandleFunc("/testing/config", PATCH_testing_config).Methods("PATCH")
	router.HandleFunc("/testing/reset", POST_testing_reset).Methods("POST")

	corsRouter := corsMiddleware(router)
	loggedRouter := accessLogger(corsRouter)

	addr := ":" + strconv.Itoa(port)
	if err := http.ListenAndServe(addr, loggedRouter); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
