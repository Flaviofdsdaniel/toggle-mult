package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type EvaluationResponse struct {
	FlagName string `json:"flag_name"`
	UserID   string `json:"user_id"`
	Result   bool   `json:"result"`
}

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("Erro ao codificar resposta JSON: %v", err)
	}
}

func (a *App) evaluationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. Parsear os query parameters
	userID := r.URL.Query().Get("user_id")
	flagName := r.URL.Query().Get("flag_name")

	if userID == "" || flagName == "" {
		http.Error(w, `{"error": "user_id e flag_name são obrigatórios"}`, http.StatusBadRequest)
		return
	}

	// 2. Obter a decisão (lógica de cache/serviço está em evaluator.go)
	result, err := a.getDecision(r.Context(), userID, flagName)
	if err != nil {
		// Se o erro for "não encontrado", retornamos 'false' (comportamento seguro)
		if _, ok := err.(*NotFoundError); ok {
			result = false
		} else {
			// Outros erros (serviços offline, etc)
			log.Printf("Erro ao avaliar flag '%s': %v", flagName, err)
			http.Error(w, `{"error": "Erro interno ao avaliar a flag"}`, http.StatusBadGateway)
			return
		}
	}

	// 3. Enviar evento para SQS (assincronamente)
	// Isso não bloqueia a resposta para o cliente.
	// Mantém o contexto do trace, mas desacopla o envio do cancelamento da
	// requisição HTTP. O timeout evita deixar a goroutine presa indefinidamente.
	eventCtx := context.WithoutCancel(r.Context())
	go func() {
		ctx, cancel := context.WithTimeout(eventCtx, 5*time.Second)
		defer cancel()
		a.sendEvaluationEvent(ctx, userID, flagName, result)
	}()

	// 4. Retornar a resposta
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(EvaluationResponse{
		FlagName: flagName,
		UserID:   userID,
		Result:   result,
	}); err != nil {
		log.Printf("Erro ao codificar resposta JSON: %v", err)
	}
}
