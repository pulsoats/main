package live

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/system"
	grpclive "github.com/pulsoats/main/internal/adapters/grpc/live"
	grpcsystem "github.com/pulsoats/main/internal/adapters/grpc/system"
	applive "github.com/pulsoats/main/internal/application/live"
)

type entry struct {
	info   system.ServiceInfo
	client applive.Client
}

type Pool struct {
	mu      sync.RWMutex
	entries map[uuid.UUID]entry
	index   map[string]uuid.UUID // exchange+":"+account → serviceID
	tlsCfg  *tls.Config
}

func NewPool(tlsCfg *tls.Config) *Pool {
	return &Pool{
		entries: make(map[uuid.UUID]entry),
		index:   make(map[string]uuid.UUID),
		tlsCfg:  tlsCfg,
	}
}

func (p *Pool) Register(ctx context.Context, addr string) (system.ServiceInfo, error) {
	sysCli, err := grpcsystem.NewClient(addr, p.tlsCfg)
	if err != nil {
		return system.ServiceInfo{}, fmt.Errorf("pool register: system client: %w", err)
	}

	info, err := sysCli.Info(ctx)
	if err != nil {
		return system.ServiceInfo{}, fmt.Errorf("pool register: info: %w", err)
	}

	liveCli, err := grpclive.NewClient(addr, p.tlsCfg)
	if err != nil {
		return system.ServiceInfo{}, fmt.Errorf("pool register: live client: %w", err)
	}

	p.mu.Lock()
	if _, exists := p.index[indexKey(info.Exchange, info.Account)]; exists {
		p.mu.Unlock()
		return system.ServiceInfo{}, fmt.Errorf("pool register: %w", errorsx.ErrAlreadyExists)
	}
	p.entries[info.ID] = entry{info: info, client: liveCli}
	p.index[indexKey(info.Exchange, info.Account)] = info.ID
	p.mu.Unlock()

	return info, nil
}

func (p *Pool) Get(id uuid.UUID) (applive.Client, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	e, ok := p.entries[id]
	return e.client, ok
}

func (p *Pool) GetByExchangeAccount(exchange, account string) (uuid.UUID, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	id, ok := p.index[indexKey(exchange, account)]
	return id, ok
}

func (p *Pool) Remove(id uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e, ok := p.entries[id]; ok {
		delete(p.index, indexKey(e.info.Exchange, e.info.Account))
		delete(p.entries, id)
	}
}

func (p *Pool) List() []applive.PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]applive.PoolEntry, 0, len(p.entries))
	for _, e := range p.entries {
		result = append(result, applive.PoolEntry{Info: e.info, Client: e.client})
	}
	return result
}

func indexKey(exchange, account string) string {
	return strings.ToLower(exchange) + ":" + strings.ToLower(account)
}
