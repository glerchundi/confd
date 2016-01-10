package config

import (
	"github.com/docker/libkv/store"
)

type BackendConfig interface {
	Type() store.Backend
	IsWatchSupported() bool
}

//
// consul
//

type ConsulBackendConfig struct {
	Endpoints []string
	CAFile    string
	CertFile  string
	KeyFile   string
}

func NewConsulBackendConfig() *ConsulBackendConfig {
	return &ConsulBackendConfig{
		Endpoints: []string{"127.0.0.1:8500"},
		CAFile:    "",
		CertFile:  "",
		KeyFile:   "",
	}
}

func (*ConsulBackendConfig) Type() store.Backend {
	return store.CONSUL
}

func (*ConsulBackendConfig) IsWatchSupported() bool {
	return true
}

//
// etcd
//

type EtcdBackendConfig struct {
	Endpoints []string
	CAFile    string
	CertFile  string
	KeyFile   string
}

func NewEtcdBackendConfig() *EtcdBackendConfig {
	return &EtcdBackendConfig{
		Endpoints: []string{"127.0.0.1:2379"},
		CAFile:    "",
		CertFile:  "",
		KeyFile:   "",
	}
}

func (*EtcdBackendConfig) Type() store.Backend {
	return store.ETCD
}

func (*EtcdBackendConfig) IsWatchSupported() bool {
	return true
}

//
// zookeeper
//

type ZookeeperBackendConfig struct {
	Endpoints []string
}

func NewZookeeperBackendConfig() *ZookeeperBackendConfig {
	return &ZookeeperBackendConfig{
		Endpoints: []string{"127.0.0.1:2181"},
	}
}

func (*ZookeeperBackendConfig) Type() store.Backend {
	return store.ZK
}

func (*ZookeeperBackendConfig) IsWatchSupported() bool {
	return true
}

/*
//
// boltdb
//

type BoltDBBackendConfig struct {
}

func NewBoltDBBackendConfig() *BoltDBBackendConfig {
	return &BoltDBBackendConfig{
	}
}

func (*BoltDBBackendConfig) Type() string {
	return string(store.BOLTDB)
}

func (*BoltDBBackendConfig) IsWatchSupported() bool {
	return false
}
*/