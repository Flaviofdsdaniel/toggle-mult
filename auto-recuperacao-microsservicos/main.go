package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	alertName       = "MicrosservicoIndisponivel"
	workloadNS      = "toggle-mult"
	serviceAccount  = "/var/run/secrets/kubernetes.io/serviceaccount"
	restartCooldown = 10 * time.Minute
)

var allowedDeployments = map[string]struct{}{
	"auth-service":       {},
	"flag-service":       {},
	"evaluation-service": {},
	"analytics-service":  {},
	"targeting-service":  {},
}

type alertmanagerWebhook struct {
	Alerts []struct {
		Status string            `json:"status"`
		Labels map[string]string `json:"labels"`
	} `json:"alerts"`
}

type restartLimiter struct {
	mu       sync.Mutex
	restarted map[string]time.Time
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	limiter := &restartLimiter{restarted: make(map[string]time.Time)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("POST /alerts", limiter.handleAlerts)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("serviço de autorrecuperação ouvindo na porta %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("servidor HTTP: %v", err)
	}
}

func (l *restartLimiter) handleAlerts(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var payload alertmanagerWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "payload do Alertmanager inválido", http.StatusBadRequest)
		return
	}

	for _, alert := range payload.Alerts {
		if alert.Status != "firing" || alert.Labels["alertname"] != alertName {
			continue
		}
		if alert.Labels["namespace"] != workloadNS {
			log.Printf("alerta ignorado: namespace não permitido")
			continue
		}

		deployment := alert.Labels["deployment"]
		if _, ok := allowedDeployments[deployment]; !ok {
			log.Printf("alerta ignorado: deployment não permitido: %q", deployment)
			continue
		}

		if err := l.restartDeployment(r.Context(), deployment); err != nil {
			log.Printf("falha ao reiniciar deployment %s/%s: %v", workloadNS, deployment, err)
			http.Error(w, "falha ao solicitar reinício do microsserviço", http.StatusBadGateway)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = io.WriteString(w, "alertas processados\n")
}

func (l *restartLimiter) restartDeployment(ctx context.Context, deployment string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()
	if previous, ok := l.restarted[deployment]; ok && now.Sub(previous) < restartCooldown {
		log.Printf("reinício ignorado para %s/%s: intervalo de segurança ativo", workloadNS, deployment)
		return nil
	}

	if err := patchDeployment(ctx, deployment, now); err != nil {
		return err
	}
	l.restarted[deployment] = now
	log.Printf("reinício solicitado para %s/%s", workloadNS, deployment)
	return nil
}

func patchDeployment(ctx context.Context, deployment string, restartedAt time.Time) error {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
	if port == "" {
		port = "443"
	}
	if host == "" {
		return fmt.Errorf("KUBERNETES_SERVICE_HOST não definido")
	}

	token, err := os.ReadFile(filepath.Join(serviceAccount, "token"))
	if err != nil {
		return fmt.Errorf("ler token da ServiceAccount: %w", err)
	}
	caPEM, err := os.ReadFile(filepath.Join(serviceAccount, "ca.crt"))
	if err != nil {
		return fmt.Errorf("ler CA do cluster: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return fmt.Errorf("CA do cluster inválida")
	}

	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}
	defer transport.CloseIdleConnections()

	patch := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]string{
						"self-healing.togglemaster/restartedAt": restartedAt.Format(time.RFC3339),
					},
				},
			},
		},
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("serializar patch: %w", err)
	}

	url := fmt.Sprintf("https://%s:%s/apis/apps/v1/namespaces/%s/deployments/%s", host, port, workloadNS, deployment)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("criar requisição Kubernetes: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	req.Header.Set("Content-Type", "application/merge-patch+json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("chamar API do Kubernetes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		response, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("API do Kubernetes retornou %s: %s", resp.Status, strings.TrimSpace(string(response)))
	}
	return nil
}
