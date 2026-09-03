module github.com/faultiq/graph-manager

go 1.24

require github.com/neo4j/neo4j-go-driver/v5 v5.13.0

require github.com/redis/go-redis/v9 v9.0.0

require github.com/faultiq/graphclient v0.0.0

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace github.com/faultiq/graphclient => ../../pkg/graphclient
