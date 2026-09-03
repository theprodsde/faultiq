// k8s-connector discovers Kubernetes services and generates TechGraph-compatible
// service map entries. It reads Service objects from a namespace and exposes
// GET /discover?namespace=default to return discovered services as JSON.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ServiceEntry mirrors services/health-poller/servicemap.ServiceEntry for JSON output.
type ServiceEntry struct {
	ID          string   `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Type        string   `json:"type" yaml:"type"`
	HealthURL   string   `json:"healthUrl" yaml:"healthUrl"`
	ClusterIP   string   `json:"clusterIp,omitempty" yaml:"clusterIp,omitempty"`
	Repo        string   `json:"repo,omitempty" yaml:"repo,omitempty"`
	Namespace   string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Calls       []string `json:"calls,omitempty" yaml:"calls,omitempty"` // techgraph.io/calls annotation
}

// newK8sClient builds a Kubernetes client.
// Priority: if KUBERNETES_SERVICE_HOST is set (injected automatically by Kubernetes into
// every pod), use in-cluster config (ServiceAccount token).  Otherwise fall back to the
// KUBECONFIG env var or the default ~/.kube/config for local / dev use.
func newK8sClient() (*kubernetes.Clientset, error) {
	var cfg *rest.Config
	var err error
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		// Running inside a Kubernetes cluster — use the mounted ServiceAccount token.
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("in-cluster config: %w", err)
		}
	} else {
		// Local / dev mode — use KUBECONFIG env var or default ~/.kube/config.
		kubeconfig := os.Getenv("KUBECONFIG")
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig: %w", err)
		}
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}
	return cs, nil
}

// discoverServices lists Kubernetes Services in the given namespace and maps them to ServiceEntries.
//
// Annotations recognized:
//   techgraph.io/health-path  — health endpoint path (default: /health)
//   techgraph.io/type         — node type: SERVICE | DATABASE | QUEUE | GATEWAY | EXTERNAL (default: SERVICE)
//   techgraph.io/service-type — legacy alias for techgraph.io/type (kept for backward compatibility)
//   techgraph.io/repo         — source repository URL
//   techgraph.io/calls        — comma-separated list of service IDs this service calls
func discoverServices(ctx context.Context, cs *kubernetes.Clientset, namespace string) ([]ServiceEntry, error) {
	list, err := cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	var entries []ServiceEntry
	for _, svc := range list.Items {
		ann := svc.Annotations
		if ann == nil {
			ann = map[string]string{}
		}

		healthPath := ann["techgraph.io/health-path"]
		if healthPath == "" {
			healthPath = "/health"
		}

		// Prefer techgraph.io/type; fall back to legacy techgraph.io/service-type.
		svcType := ann["techgraph.io/type"]
		if svcType == "" {
			svcType = ann["techgraph.io/service-type"]
		}
		if svcType == "" {
			svcType = "SERVICE"
		}

		repo := ann["techgraph.io/repo"]

		// Parse comma-separated calls list from techgraph.io/calls annotation.
		var calls []string
		if callsStr := ann["techgraph.io/calls"]; callsStr != "" {
			for _, c := range strings.Split(callsStr, ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					calls = append(calls, c)
				}
			}
		}

		// Determine health URL: use ClusterIP if available, otherwise service DNS name.
		host := svc.Spec.ClusterIP
		if host == "" || host == "None" {
			host = fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace)
		}

		// Find the first HTTP-like port.
		port := 80
		for _, p := range svc.Spec.Ports {
			if p.Port > 0 {
				port = int(p.Port)
				break
			}
		}

		healthURL := fmt.Sprintf("http://%s:%d%s", host, port, healthPath)

		entries = append(entries, ServiceEntry{
			ID:        svc.Name,
			Name:      svc.Name,
			Type:      svcType,
			HealthURL: healthURL,
			ClusterIP: svc.Spec.ClusterIP,
			Repo:      repo,
			Namespace: svc.Namespace,
			Calls:     calls,
		})
	}
	return entries, nil
}

func discoverHandler(cs *kubernetes.Clientset, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey != "" && r.Header.Get("X-API-Key") != apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = os.Getenv("DEFAULT_NAMESPACE")
		}
		if namespace == "" {
			namespace = "default"
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		entries, err := discoverServices(ctx, cs, namespace)
		if err != nil {
			http.Error(w, fmt.Sprintf("discovery failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"namespace": namespace,
			"services":  entries,
			"count":     len(entries),
		})
	}
}

func main() {
	// Build K8s client — if not available (outside cluster, no kubeconfig), start in degraded mode.
	// The /discover endpoint returns 503 with a helpful message instead of crashing.
	cs, k8sErr := newK8sClient()
	if k8sErr != nil {
		log.Printf("k8s-connector: K8s unavailable (%v) — starting in degraded mode (set KUBECONFIG or run in-cluster)", k8sErr)
	}

	port := os.Getenv("K8S_CONNECTOR_PORT")
	if port == "" {
		port = "8093"
	}
	apiKey := os.Getenv("K8S_CONNECTOR_API_KEY")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if k8sErr != nil {
			w.WriteHeader(http.StatusOK) // healthy as a service, even if K8s is unavailable
			_, _ = w.Write([]byte(`{"status":"ok","k8s":"unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","k8s":"connected"}`))
	})
	mux.HandleFunc("/discover", func(w http.ResponseWriter, r *http.Request) {
		if k8sErr != nil {
			http.Error(w, `{"error":"K8s not configured — set KUBECONFIG env or run inside a cluster"}`, http.StatusServiceUnavailable)
			return
		}
		discoverHandler(cs, apiKey)(w, r)
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	log.Printf("k8s-connector: listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("k8s-connector: %v", err)
	}
}
