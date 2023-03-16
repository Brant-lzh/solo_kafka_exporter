package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

const (
	CLEARINTERVAL = 6
)

var cache *cacheRegistry

type cacheRegistry struct {
	mp            map[string]*multiRegistry
	mutex         *sync.RWMutex
	clearInterval int //hour
}

func init() {
	cache = &cacheRegistry{
		mp:            make(map[string]*multiRegistry),
		mutex:         &sync.RWMutex{},
		clearInterval: CLEARINTERVAL,
	}
	go cache.TimingClearRegistry()
}

func (c *cacheRegistry) TimingClearRegistry() {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	for key, registry := range c.mp {
		if time.Now().Sub(registry.UpdateTime).Hours() > float64(c.clearInterval) {
			delete(c.mp, key)
			klog.V(INFO).Infoln("Delete Registry", key)
		}
	}
}

func (c *cacheRegistry) Get(key string) (*multiRegistry, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	result, found := c.mp[key]
	return result, found
}

func (c *cacheRegistry) Set(key string, value *multiRegistry) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.mp[key] = value
}

func (c *cacheRegistry) Count() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.mp)
}

type multiRegistry struct {
	registry   *prometheus.Registry
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
}

func NewMultiRegistry(cs prometheus.Collector) *multiRegistry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(cs)
	return &multiRegistry{
		registry:   registry,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
}

func (m *multiRegistry) SetUpdateTime(time time.Time) {
	m.UpdateTime = time
}
