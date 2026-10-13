package machinery

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	redisbackend "github.com/dragonflyoss/machinery/v1/backends/redis"
	redisbroker "github.com/dragonflyoss/machinery/v1/brokers/redis"
	"github.com/dragonflyoss/machinery/v1/config"
	"github.com/redis/go-redis/v9"
)

func TestRedisConnectionOptions(t *testing.T) {
	settings := &config.RedisConfig{MasterName: "mymaster", ConnectTimeout: 60}
	user, password := "worker +:@", "test @:/%+"
	addrs := []string{"sentinel-1:26379", "sentinel-2:26379", "sentinel-3:26379"}
	for _, db := range []int{11, 12} {
		raw := (&url.URL{Scheme: "redis", Host: strings.Join(addrs, ","), Path: fmt.Sprintf("/%d", db), User: url.UserPassword(user, password)}).String()
		opts, err := redisConnectionOptions(raw, settings)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(opts.Addrs, addrs) || opts.DB != db || opts.MasterName != "mymaster" ||
			opts.Username != user || opts.Password != password || opts.DialTimeout != 60*time.Second {
			t.Fatal("endpoint, authentication or Sentinel database was lost")
		}
	}
	for _, tc := range []struct {
		raw      string
		settings *config.RedisConfig
	}{
		{"redis://a:6381,b:6382/11", nil},
		{"redis://a:6381/not-a-db", nil},
		{"redis://a:6381,b:6382/0/1", nil},
		{"redis://a:70000/0", nil},
		{"redis://a:6381/0", &config.RedisConfig{MasterName: "mymaster", ClusterMode: true}},
	} {
		if _, err := redisConnectionOptions(tc.raw, tc.settings); err == nil {
			t.Fatal("invalid connection accepted")
		}
	}
	if _, err := redisConnectionOptions("redis://a:6381,b:6382/0", nil); err != nil {
		t.Fatal(err)
	}
	opts, err := redisConnectionOptions("redis://password@localhost:6379/12", nil)
	if err != nil || opts.DB != 12 || opts.Password != "password" || opts.Username != "" {
		t.Fatal("single-node compatibility broken")
	}
	opts, err = redisConnectionOptions("redis://test+password@localhost:6379/0", nil)
	if err != nil || opts.Password != "test+password" {
		t.Fatal("literal plus in URL userinfo was converted to a space")
	}
}

func TestRedisFactoryCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name      string
		raw       string
		settings  *config.RedisConfig
		universal bool
	}{
		{"standalone", "redis://localhost:6379/12", nil, false},
		{"sentinel", "redis://a:26379,b:26379/11", &config.RedisConfig{MasterName: "mymaster"}, true},
		{"cluster", "redis://a:6381,b:6382/0", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cnf := &config.Config{Broker: tc.raw, ResultBackend: tc.raw, Redis: tc.settings}
			broker, err := BrokerFactory(cnf)
			if err != nil {
				t.Fatal(err)
			}
			_, universalBroker := broker.(*redisbroker.BrokerGR)
			backend, err := BackendFactory(cnf)
			if err != nil {
				t.Fatal(err)
			}
			_, universalBackend := backend.(*redisbackend.BackendGR)
			if universalBroker != tc.universal || universalBackend != tc.universal {
				t.Fatal("Redis factory selected an incompatible implementation")
			}
		})
	}
	if _, err := NewServer(&config.Config{
		Broker:        "redis://a:6381,b:6382/0",
		ResultBackend: "redis://a:6381,b:6382/11",
	}); err == nil {
		t.Fatal("invalid configured backend was silently accepted")
	}
	client := newRedisConnectionClient(&redis.UniversalOptions{Addrs: []string{"localhost:6381"}}, &config.RedisConfig{ClusterMode: true})
	defer client.Close()
	if _, ok := client.(*redis.ClusterClient); !ok {
		t.Fatal("explicit single-endpoint Cluster mode was ignored")
	}
}
