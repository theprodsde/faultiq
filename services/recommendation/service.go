package main

import (
    "encoding/json"
    "io/ioutil"
    "net/http"
    "strings"
)

type Playbook struct {
    ID    string `json:"id"`
    Title string `json:"title"`
}

type Store struct {
    mapping map[string][]Playbook
}

func NewStoreFromFile(path string) (*Store, error) {
    b, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var m map[string][]Playbook
    if err := json.Unmarshal(b, &m); err != nil {
        return nil, err
    }
    return &Store{mapping: m}, nil
}

func (s *Store) Recommend(symptom string) []Playbook {
    if p, ok := s.mapping[symptom]; ok {
        return p
    }
    // Fuzzy prefix match for backward compatibility
    for key, p := range s.mapping {
        if strings.HasPrefix(key, symptom) || strings.HasPrefix(symptom, key) {
            return p
        }
    }
    return nil
}

func (s *Store) Handler() http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"ok"}`))
    })
    mux.HandleFunc("/recommend", func(w http.ResponseWriter, r *http.Request) {
        symptom := r.URL.Query().Get("symptom")
        plays := s.Recommend(symptom)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(plays)
    })
    return mux
}
