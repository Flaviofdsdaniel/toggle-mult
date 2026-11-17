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
	"time"
)

const (
	alertName      = "RollbackVersaoDeDeploy"
	workloadNS     = "toggle-mult"
	defaultPort    = "8080"
	defaultToken   = "/var/run/secrets/github/token"
	defaultRepo    = "Flaviofdsdaniel/toggle-mult"
	serviceAccount = "/var/run/secrets/kubernetes.io/serviceaccount"
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

type deploymentImageResolver interface {
	currentImage(context.Context, string) (string, bool, error)
}

type rollbackDispatcher interface {
	dispatch(context.Context, string, string, string) error
}

type alertHandler struct {
	images     deploymentImageResolver
	dispatcher rollbackDispatcher
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	tokenPath := os.Getenv("GITHUB_TOKEN_PATH")
	if tokenPath == "" {
		tokenPath = defaultToken
	}
	repository := os.Getenv("GITHUB_REPOSITORY")
	if repository == "" {
		repository = defaultRepo
	}
	apiURL := os.Getenv("GITHUB_API_URL")
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}

	kubeClient, err := newInClusterClient()
	if err != nil {
		log.Fatalf("configuração do cliente Kubernetes: %v", err)
	}

	handler := &alertHandler{
		images: kubeClient,
		dispatcher: &githubDispatcher{
			client:     &http.Client{Timeout: 15 * time.Second},
			tokenPath:  tokenPath,
			repository: repository,
			apiURL:     strings.TrimRight(apiURL, "/"),
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		if strings.TrimSpace(readFile(tokenPath)) == "" {
			http.Error(w, "token do GitHub ainda não está disponível", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ready\n")
	})
	mux.Handle("POST /alerts", handler)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("webhook de rollback GitOps ouvindo na porta %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("servidor HTTP: %v", err)
	}
}

func (h *alertHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var payload alertmanagerWebhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "payload do Alertmanager inválido", http.StatusBadRequest)
		return
	}

	processed := 0
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
			log.Printf("alerta ignorado: microsserviço não permitido: %q", deployment)
			continue
		}

		image, fullyRolledOut, err := h.images.currentImage(r.Context(), deployment)
		if err != nil {
			log.Printf("falha ao consultar imagem atual de %s/%s: %v", workloadNS, deployment, err)
			http.Error(w, "falha ao consultar a versão atual do microsserviço", http.StatusBadGateway)
			return
		}
		if strings.TrimSpace(image) == "" {
			log.Printf("alerta ignorado: imagem atual ausente para %s/%s", workloadNS, deployment)
			continue
		}
		if fullyRolledOut {
			log.Printf("rollback ignorado: %s/%s já está na versão desejada e saudável", workloadNS, deployment)
			continue
		}

		if err := h.dispatcher.dispatch(r.Context(), deployment, image, alertName); err != nil {
			log.Printf("falha ao acionar rollback GitOps de %s: %v", deployment, err)
			http.Error(w, "falha ao acionar a GitHub Action de rollback", http.StatusBadGateway)
			return
		}
		processed++
		log.Printf("rollback GitOps solicitado para %s/%s; imagem observada: %s", workloadNS, deployment, image)
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = fmt.Fprintf(w, "alertas processados: %d\n", processed)
}

type githubDispatcher struct {
	client     *http.Client
	tokenPath  string
	repository string
	apiURL     string
}

func (d *githubDispatcher) dispatch(ctx context.Context, service, expectedImage, name string) error {
	token := strings.TrimSpace(readFile(d.tokenPath))
	if token == "" {
		return fmt.Errorf("token do GitHub não configurado")
	}

	payload := map[string]any{
		"event_type": "rollback-deployment",
		"client_payload": map[string]string{
			"service":        service,
			"namespace":      workloadNS,
			"alertname":      name,
			"expected_image": expectedImage,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serializar evento: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/dispatches", d.apiURL, d.repository)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("criar requisição ao GitHub: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("chamar API do GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		response, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("API do GitHub retornou %s: %s", resp.Status, strings.TrimSpace(string(response)))
	}
	return nil
}

type inClusterClient struct {
	host      string
	port      string
	tokenPath string
	client    *http.Client
}

func newInClusterClient() (*inClusterClient, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	if host == "" {
		return nil, fmt.Errorf("KUBERNETES_SERVICE_HOST não definido")
	}
	port := os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
	if port == "" {
		port = "443"
	}
	caPEM, err := os.ReadFile(filepath.Join(serviceAccount, "ca.crt"))
	if err != nil {
		return nil, fmt.Errorf("ler CA do cluster: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("CA do cluster inválida")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	return &inClusterClient{
		host:      host,
		port:      port,
		tokenPath: filepath.Join(serviceAccount, "token"),
		client:    &http.Client{Transport: transport, Timeout: 15 * time.Second},
	}, nil
}

func (c *inClusterClient) currentImage(ctx context.Context, deployment string) (string, bool, error) {
	url := fmt.Sprintf("https://%s:%s/apis/apps/v1/namespaces/%s/deployments/%s", c.host, c.port, workloadNS, deployment)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, fmt.Errorf("criar consulta ao Kubernetes: %w", err)
	}
	return c.getImage(req)
}

func (c *inClusterClient) getImage(req *http.Request) (string, bool, error) {
	token := strings.TrimSpace(readFile(c.tokenPath))
	if token == "" {
		return "", false, fmt.Errorf("token da ServiceAccount ausente")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("consultar API do Kubernetes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		response, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", false, fmt.Errorf("API do Kubernetes retornou %s: %s", resp.Status, strings.TrimSpace(string(response)))
	}
	var deployment struct {
		Spec struct {
			Replicas *int32 `json:"replicas"`
			Template struct {
				Spec struct {
					Containers []struct {
						Name  string `json:"name"`
						Image string `json:"image"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
		Status struct {
			Replicas          int32 `json:"replicas"`
			UpdatedReplicas   int32 `json:"updatedReplicas"`
			ReadyReplicas     int32 `json:"readyReplicas"`
			AvailableReplicas int32 `json:"availableReplicas"`
		} `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&deployment); err != nil {
		return "", false, fmt.Errorf("decodificar Deployment: %w", err)
	}
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	fullyRolledOut := replicas == 0 || (deployment.Status.Replicas == replicas &&
		deployment.Status.UpdatedReplicas >= replicas &&
		deployment.Status.ReadyReplicas >= replicas &&
		deployment.Status.AvailableReplicas >= replicas)
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == deploymentNameFromURL(req.URL.Path) {
			return container.Image, fullyRolledOut, nil
		}
	}
	if len(deployment.Spec.Template.Spec.Containers) == 1 {
		return deployment.Spec.Template.Spec.Containers[0].Image, fullyRolledOut, nil
	}
	return "", false, fmt.Errorf("container do Deployment não encontrado")
}

func deploymentNameFromURL(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
