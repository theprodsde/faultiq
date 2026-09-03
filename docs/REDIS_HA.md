# Redis High Availability with Sentinel

FaultIQ ships with a standard single-node Redis for development. For production, use the optional Sentinel overlay to get automatic failover.

---

## Architecture

```
          ┌──────────────┐
          │   redis       │  primary (6379)
          └──────┬───────┘
                 │ replication
          ┌──────▼───────┐
          │ redis-replica │  replica (6379)
          └──────────────┘

  sentinel-1 :26379   sentinel-2 :26380
  (monitor both, quorum = 2)
```

Sentinels monitor the primary. If the primary misses heartbeats for `down-after-milliseconds` (5 s default), both sentinels vote to promote the replica and update the `mymaster` pointer. Clients that use the Sentinel protocol discover the new primary automatically.

---

## Starting HA mode

```bash
docker compose \
  -f docker-compose.yml \
  --profile redis-ha \
  up -d
```

This extends the base `redis` service with AOF persistence and adds the replica plus two sentinels.

---

## Configuring REDIS_ADDR for Sentinel

Standard Redis clients (single-address mode) do not support Sentinel natively. Use a Sentinel-aware connection string or configure the service to use a Sentinel client.

**Option A — Sentinel-aware client (Go `go-redis`):**

```go
rdb := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{
        "redis-sentinel-1:26379",
        "redis-sentinel-2:26380",
    },
})
```

Set the environment variables:

```env
REDIS_SENTINEL_ADDRS=redis-sentinel-1:26379,redis-sentinel-2:26380
REDIS_SENTINEL_MASTER=mymaster
```

**Option B — Single-address fallback (no code changes)**

If HA failover is not required during planned maintenance but you still want a replica for reads, set `REDIS_ADDR` to the primary as usual:

```env
REDIS_ADDR=redis:6379
```

Traffic falls back to the replica only after Sentinel promotes it and the client reconnects.

---

## Persistence

The HA overlay enables AOF persistence on both primary and replica:

```
command: redis-server --appendonly yes
```

Data survives container restarts. To also enable RDB snapshots, add `--save 60 1` to the command.

---

## Health check

Sentinels expose a `PING`/`INFO` interface. Verify sentinel state:

```bash
redis-cli -p 26379 sentinel masters
redis-cli -p 26379 sentinel slaves mymaster
```

---

## Disabling HA

To revert to single-node Redis, start without the overlay:

```bash
docker compose -f docker-compose.yml up -d
```
