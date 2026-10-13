package machinery

import (
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dragonflyoss/machinery/v1/config"
	"github.com/redis/go-redis/v9"
)

// redisConnectionOptions separates the database path before splitting endpoints.
// Error messages intentionally omit the URL, which can contain credentials.
func redisConnectionOptions(raw string, settings *config.RedisConfig) (*redis.UniversalOptions, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "redis" && u.Scheme != "rediss") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid Redis connection URL")
	}
	opts := &redis.UniversalOptions{Addrs: strings.Split(u.Host, ",")}
	for _, addr := range opts.Addrs {
		host, port, err := net.SplitHostPort(addr)
		if err != nil || host == "" {
			return nil, errors.New("Redis endpoints must use host:port format")
		}
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return nil, errors.New("invalid Redis endpoint port")
		}
	}
	if u.Path != "" {
		opts.DB, err = strconv.Atoi(strings.TrimPrefix(u.Path, "/"))
		if err != nil || opts.DB < 0 {
			return nil, errors.New("invalid Redis database")
		}
	}
	// URL userinfo uses percent decoding; a literal '+' is not a space.
	if u.User != nil {
		opts.Username = u.User.Username()
		var hasPassword bool
		opts.Password, hasPassword = u.User.Password()
		if !hasPassword {
			opts.Password, opts.Username = opts.Username, ""
		}
	}
	forceCluster := false
	if settings != nil {
		opts.MasterName = settings.MasterName
		forceCluster = settings.ClusterMode
		if settings.ConnectTimeout > 0 {
			opts.DialTimeout = time.Duration(settings.ConnectTimeout) * time.Second
		}
		if settings.ReadTimeout > 0 {
			opts.ReadTimeout = time.Duration(settings.ReadTimeout) * time.Second
		}
		if settings.WriteTimeout > 0 {
			opts.WriteTimeout = time.Duration(settings.WriteTimeout) * time.Second
		}
	}
	if forceCluster && opts.MasterName != "" {
		return nil, errors.New("Redis Cluster mode cannot be combined with Sentinel masterName")
	}
	if opts.MasterName == "" && (len(opts.Addrs) > 1 || forceCluster) && opts.DB != 0 {
		return nil, errors.New("Redis Cluster requires database 0")
	}
	if u.Scheme == "rediss" {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opts, nil
}

func newRedisConnectionClient(opts *redis.UniversalOptions, settings *config.RedisConfig) redis.UniversalClient {
	if settings != nil && settings.ClusterMode {
		return redis.NewClusterClient(opts.Cluster())
	}
	return redis.NewUniversalClient(opts)
}
