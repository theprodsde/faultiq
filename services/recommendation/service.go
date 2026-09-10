package main

import (
    "encoding/json"
    "io/ioutil"
    "net/http"
    "sort"
    "strings"
)

type Playbook struct {
    ID    string `json:"id"`
    Title string `json:"title"`
}

type Store struct {
    mapping    map[string][]Playbook
    sortedKeys []string
    ac         *AhoCorasick
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
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    patterns := make([]string, 0, len(m))
    for k := range m {
        patterns = append(patterns, k)
    }
    return &Store{mapping: m, sortedKeys: keys, ac: NewAhoCorasick(patterns)}, nil
}

func (s *Store) Recommend(symptom string) []Playbook {
    // 1. Exact match (fastest path)
    if p, ok := s.mapping[symptom]; ok {
        return p
    }

    // 2. Aho-Corasick scan — finds ALL patterns that appear as substrings of symptom.
    //    Merges playbooks for every distinct matching key, enabling compound symptoms like
    //    "high-latency+connection-refused" to match both "high-latency" and "connection-refused".
    if s.ac != nil {
        seen := make(map[string]struct{})
        var results []Playbook
        for _, key := range s.ac.Search(symptom) {
            if _, already := seen[key]; already {
                continue
            }
            seen[key] = struct{}{}
            results = append(results, s.mapping[key]...)
        }
        if len(results) > 0 {
            return results
        }
    }

    // 3. Binary-search prefix fallback (kept for backward compatibility)
    i := sort.SearchStrings(s.sortedKeys, symptom)
    // check forward: symptom is a prefix of sortedKeys[i]
    if i < len(s.sortedKeys) && strings.HasPrefix(s.sortedKeys[i], symptom) {
        return s.mapping[s.sortedKeys[i]]
    }
    // check backward: sortedKeys[i-1] is a prefix of symptom
    if i > 0 && strings.HasPrefix(symptom, s.sortedKeys[i-1]) {
        return s.mapping[s.sortedKeys[i-1]]
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
