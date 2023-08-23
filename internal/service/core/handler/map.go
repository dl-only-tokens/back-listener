package handler

import (
	"github.com/dl-only-tokens/back-listener/internal/service/core/listener"
	"sync"
)

type ListenersMap struct {
	mx   sync.RWMutex
	data map[listener.Listener]bool
}

func NewCounters() *ListenersMap {
	return &ListenersMap{
		data: make(map[listener.Listener]bool),
	}
}

func (c *ListenersMap) Load(key listener.Listener) (bool, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()
	val, ok := c.data[key]
	return val, ok
}

func (c *ListenersMap) Store(key listener.Listener, value bool) {
	c.mx.Lock()
	defer c.mx.Unlock()
	c.data[key] = value
}

func (c *ListenersMap) GetCopy() map[listener.Listener]bool {
	c.mx.Lock()
	defer c.mx.Unlock()
	return c.data
}
