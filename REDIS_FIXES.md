# Redis connection fixes

## Scope

- Parse authentication and the database path before splitting the endpoint list.
  A URL such as `redis://redis-1:6381,redis-2:6382/0` must not treat `6382/0`
  as a port number.
- Use standard URL userinfo decoding. A literal `+` in a password remains `+`;
  callers should construct credentials with `url.UserPassword`, not query escaping.
- Preserve the Sentinel master name, data-server credentials, timeouts, and
  database. A single Sentinel endpoint also selects the failover client.
- Reject nonzero Redis Cluster databases and mutually exclusive Cluster mode
  and Sentinel configuration. Honor explicit Cluster mode with one endpoint.
- Retain the existing implementation for plain standalone Redis. Return an
  error for an invalid configured Redis result backend instead of silently
  leaving the backend unset.

## Verification

Run the focused connection and factory regression tests:

```sh
go test ./v1 -run 'TestRedisConnectionOptions|TestRedisFactoryCompatibility' -count=1
```

These tests do not require a running Redis server and are not a substitute for
end-to-end validation of Redis Cluster or Sentinel. This patch does not change
Dragonfly scheduler key formats, image caches, or host-reporting lifecycle behavior.
