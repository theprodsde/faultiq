package main

import (
    "log"
    "net/http"
    "os"
)

func main() {
    path := "recommendations.json"
    if p := os.Getenv("RECOMMENDATIONS_PATH"); p != "" {
        path = p
    }
    s, err := NewStoreFromFile(path)
    if err != nil {
        log.Fatalf("failed to load recommendations: %v", err)
    }
    log.Println("recommendation service listening :8090")
    log.Fatal(http.ListenAndServe(":8090", s.Handler()))
}
