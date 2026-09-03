package main

import (
    "encoding/json"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "testing"
    "time"
)

func run(t *testing.T, dir string, name string, args ...string) {
    t.Logf("run: %s %v (cwd=%s)", name, args, dir)
    cmd := exec.Command(name, args...)
    if dir != "" {
        cmd.Dir = dir
    }
    out, err := cmd.CombinedOutput()
    t.Logf("output: %s", string(out))
    if err != nil {
        t.Fatalf("command failed: %v", err)
    }
}

func runCapture(t *testing.T, dir string, name string, args ...string) string {
    t.Logf("capture: %s %v (cwd=%s)", name, args, dir)
    cmd := exec.Command(name, args...)
    if dir != "" {
        cmd.Dir = dir
    }
    out, err := cmd.CombinedOutput()
    t.Logf("output: %s", string(out))
    if err != nil {
        t.Logf("command error: %v", err)
    }
    return string(out)
}

func httpGet(t *testing.T, url string) ([]byte, int) {
    t.Logf("GET %s", url)
    resp, err := http.Get(url)
    if err != nil {
        t.Logf("http get error: %v", err)
        return nil, 0
    }
    defer resp.Body.Close()
    b, _ := io.ReadAll(resp.Body)
    return b, resp.StatusCode
}

func TestIntegration_EventTriggersIncident(t *testing.T) {
    // Determine repo root (one level up from scripts)
    cwd, err := os.Getwd()
    if err != nil {
        t.Fatal(err)
    }
    repoRoot := filepath.Clean(filepath.Join(cwd, ".."))

    // Start (and build) compose services
    run(t, repoRoot, "docker-compose", "up", "-d", "--build", "neo4j", "api-gateway")

    // Wait for api-gateway to be healthy by curling it from inside the compose network
    ok := false
    for i := 0; i < 30; i++ {
        out := runCapture(t, "", "docker", "run", "--rm", "--network", "techgraph_default", "curlimages/curl:8.1.2", "-sS", "http://techgraph-api-gateway-1:8080/health")
        if out == "ok" || out == "ok\n" {
            ok = true
            break
        }
        time.Sleep(1 * time.Second)
    }
    if !ok {
        t.Fatal("api-gateway not responding inside network")
    }

    // Seed Neo4j using the same docker-run approach as manual steps
    // Use the repo absolute path for mounting
    run(t, "", "docker", "run", "--rm", "--network", "techgraph_default", "-v", repoRoot+"/scripts:/work", "-w", "/work", "golang:1.24", "sh", "-c", "go mod download && NEO4J_URI=neo4j://techgraph-neo4j-1:7687 NEO4J_PASS=testpass123 go run .")

    // Run the generator inside a container on the compose network so it can reach services by hostname
    run(t, "", "docker", "run", "--rm", "--network", "techgraph_default", "-v", repoRoot+"/scripts:/work", "-w", "/work", "golang:1.24", "sh", "-c", "go mod download && NEO4J_URI=neo4j://techgraph-neo4j-1:7687 NEO4J_PASS=testpass123 API_GATEWAY=http://techgraph-api-gateway-1:8080/api/v1/signals go run ./cmd/generator")

    // Poll incidents endpoint up to 3 minutes
    deadline := time.Now().Add(3 * time.Minute)
    for time.Now().Before(deadline) {
        out := runCapture(t, "", "docker", "run", "--rm", "--network", "techgraph_default", "curlimages/curl:8.1.2", "-sS", "http://techgraph-api-gateway-1:8080/api/v1/incidents")
        var arr []map[string]interface{}
        if out != "" {
            _ = json.Unmarshal([]byte(out), &arr)
            if len(arr) > 0 {
                t.Logf("found incidents: %d", len(arr))
                return
            }
        }
        time.Sleep(2 * time.Second)
    }
    t.Fatal("no incident detected within 3 minutes")
}
