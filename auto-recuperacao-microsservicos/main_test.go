package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeImageResolver struct {
	image          string
	fullyRolledOut bool
	deployment     string
	err            error
}

func (f *fakeImageResolver) currentImage(_ context.Context, deployment string) (string, bool, error) {
	f.deployment = deployment
	return f.image, f.fullyRolledOut, f.err
}

type dispatchedEvent struct {
	service       string
	expectedImage string
	alertName     string
}

type fakeDispatcher struct {
	events []dispatchedEvent
	err    error
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func (f *fakeDispatcher) dispatch(_ context.Context, service, expectedImage, name string) error {
	f.events = append(f.events, dispatchedEvent{service: service, expectedImage: expectedImage, alertName: name})
	return f.err
}

func TestAlertHandlerDispatchesAllowedDeployment(t *testing.T) {
	images := &fakeImageResolver{image: "registry/auth-service:release-2"}
	dispatcher := &fakeDispatcher{}
	handler := &alertHandler{images: images, dispatcher: dispatcher}
	body := `{"alerts":[{"status":"firing","labels":{"alertname":"RollbackVersaoDeDeploy","namespace":"toggle-mult","deployment":"auth-service"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(body))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, esperado %d; body=%s", resp.Code, http.StatusAccepted, resp.Body.String())
	}
	if images.deployment != "auth-service" {
		t.Fatalf("deployment consultado = %q", images.deployment)
	}
	if len(dispatcher.events) != 1 {
		t.Fatalf("eventos enviados = %d, esperado 1", len(dispatcher.events))
	}
	if dispatcher.events[0] != (dispatchedEvent{
		service:       "auth-service",
		expectedImage: "registry/auth-service:release-2",
		alertName:     alertName,
	}) {
		t.Fatalf("evento enviado inesperado: %#v", dispatcher.events[0])
	}
}

func TestAlertHandlerIgnoresUnknownServiceAndResolvedAlert(t *testing.T) {
	images := &fakeImageResolver{image: "registry/auth-service:release-2"}
	dispatcher := &fakeDispatcher{}
	handler := &alertHandler{images: images, dispatcher: dispatcher}
	body := `{"alerts":[` +
		`{"status":"firing","labels":{"alertname":"RollbackVersaoDeDeploy","namespace":"toggle-mult","deployment":"test-deployment"}},` +
		`{"status":"resolved","labels":{"alertname":"RollbackVersaoDeDeploy","namespace":"toggle-mult","deployment":"auth-service"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(body))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, esperado %d", resp.Code, http.StatusAccepted)
	}
	if len(dispatcher.events) != 0 {
		t.Fatalf("alertas inválidos dispararam %d eventos", len(dispatcher.events))
	}
}

func TestAlertHandlerSkipsAlreadyRolledOutDeployment(t *testing.T) {
	images := &fakeImageResolver{image: "registry/auth-service:release-2", fullyRolledOut: true}
	dispatcher := &fakeDispatcher{}
	handler := &alertHandler{images: images, dispatcher: dispatcher}
	body := `{"alerts":[{"status":"firing","labels":{"alertname":"RollbackVersaoDeDeploy","namespace":"toggle-mult","deployment":"auth-service"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(body))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, esperado %d", resp.Code, http.StatusAccepted)
	}
	if len(dispatcher.events) != 0 {
		t.Fatalf("deployment já recuperado disparou %d rollbacks", len(dispatcher.events))
	}
}

func TestAlertHandlerReturnsBadGatewayWhenImageCannotBeRead(t *testing.T) {
	handler := &alertHandler{
		images:     &fakeImageResolver{err: errors.New("sem permissão")},
		dispatcher: &fakeDispatcher{},
	}
	body := `{"alerts":[{"status":"firing","labels":{"alertname":"RollbackVersaoDeDeploy","namespace":"toggle-mult","deployment":"auth-service"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(body))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, esperado %d", resp.Code, http.StatusBadGateway)
	}
}

func TestGitHubDispatcherSendsRepositoryDispatch(t *testing.T) {
	var received struct {
		EventType     string            `json:"event_type"`
		ClientPayload map[string]string `json:"client_payload"`
	}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("método = %s, esperado POST", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/dispatches" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decodificar payload: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})}

	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("test-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dispatcher := &githubDispatcher{
		client:     client,
		tokenPath:  tokenPath,
		repository: "owner/repo",
		apiURL:     "https://api.github.test",
	}

	if err := dispatcher.dispatch(context.Background(), "auth-service", "registry/auth-service:v2", alertName); err != nil {
		t.Fatalf("dispatch falhou: %v", err)
	}
	if received.EventType != "rollback-deployment" {
		t.Fatalf("event_type = %q", received.EventType)
	}
	if received.ClientPayload["service"] != "auth-service" || received.ClientPayload["expected_image"] != "registry/auth-service:v2" {
		t.Fatalf("client_payload inesperado: %#v", received.ClientPayload)
	}
}

func TestKubernetesImageResolverReadsDeploymentImage(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/apis/apps/v1/namespaces/toggle-mult/deployments/auth-service" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer kube-token" {
			t.Errorf("Authorization = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"spec":{"replicas":1,"template":{"spec":{"containers":[{"name":"auth-service","image":"registry/auth-service:v2"}]}}},"status":{"replicas":1,"updatedReplicas":1,"readyReplicas":1,"availableReplicas":1}}`,
			)),
			Header:  make(http.Header),
			Request: r,
		}, nil
	})}

	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("kube-token"), 0600); err != nil {
		t.Fatal(err)
	}
	kubeClient := &inClusterClient{tokenPath: tokenPath, client: client}
	req, err := http.NewRequest(http.MethodGet, "https://kube.local/apis/apps/v1/namespaces/toggle-mult/deployments/auth-service", nil)
	if err != nil {
		t.Fatal(err)
	}

	image, fullyRolledOut, err := kubeClient.getImage(req)
	if err != nil {
		t.Fatalf("getImage falhou: %v", err)
	}
	if image != "registry/auth-service:v2" {
		t.Fatalf("imagem = %q", image)
	}
	if !fullyRolledOut {
		t.Fatal("esperava Deployment totalmente atualizado e saudável")
	}
}
