package ratelimiter

import (
	"context"
	"log"
	"sync"
	"time"

	entity "github.com/marcioaraujo/pos-ratelimit/internal/entity"
	db "github.com/marcioaraujo/pos-ratelimit/internal/infra/database"
	"golang.org/x/time/rate"
)

type RateLimiterConfigs struct {
	BlockingDuration   time.Duration
	IpMaxReqsPerSecond int
	TokenConfigs       map[string]int
}

type RateLimiter struct {
	ctx           context.Context
	Configs       RateLimiterConfigs
	Repository    db.RateLimiterRepository
	activeClients ActiveClients
}

type ActiveClients struct {
	mu      sync.Mutex
	clients map[string]entity.ActiveClient
}

func NewRateLimiter(
	ctx context.Context,
	Configs RateLimiterConfigs,
	Repository db.RateLimiterRepository) *RateLimiter {

	rateLimiter := &RateLimiter{
		ctx:        ctx,
		Configs:    Configs,
		Repository: Repository,
		activeClients: ActiveClients{
			clients: make(map[string]entity.ActiveClient)}}

	rateLimiter.loadActiveClients()

	// Unblock client after expiration time
	go func() {
		for {
			select {
			case <-ctx.Done():
				{
					log.Println("Stopped expired blockings manager...")
					return
				}
			default:
				{
					time.Sleep(1 * time.Second)

					now := time.Now()

					clientsToUnblock := make([]string, 0)

					rateLimiter.activeClients.mu.Lock()
					for k, v := range rateLimiter.activeClients.clients {
						if v.Blocked && now.After(v.BlockedUntil) {
							clientsToUnblock = append(clientsToUnblock, k)
						}
					}
					rateLimiter.activeClients.mu.Unlock()

					for _, v := range clientsToUnblock {
						rateLimiter.unblockActiveClient(v)
					}
				}

			}

		}
	}()

	// Remove inactive clients
	go func() {
		for {
			select {
			case <-ctx.Done():
				{
					log.Println("Stopped inactive clients manager...")
					return
				}
			default:
				{
					time.Sleep(3 * time.Minute)

					clientsToRemove := make([]entity.ActiveClient, 0)

					rateLimiter.activeClients.mu.Lock()
					for k := range rateLimiter.activeClients.clients {
						client := rateLimiter.activeClients.clients[k]
						if time.Since(client.LastSeen) > 3*time.Minute {
							clientsToRemove = append(clientsToRemove, client)
						}
					}
					rateLimiter.activeClients.mu.Unlock()

					for _, v := range clientsToRemove {
						rateLimiter.removeActiveClient(v)
					}
				}
			}
		}
	}()

	return rateLimiter
}

func (r *RateLimiter) loadActiveClients() {

	log.Println("Loading active clients...")

	activeClients, err := r.Repository.GetActiveClients()
	if err != nil {
		log.Println("Error loading active clients. Starting clean.", err)
		activeClients = make(map[string]entity.ActiveClient)
	}

	log.Printf("%d active clients loaded\n", len(activeClients))

	// populate limiters
	for k := range activeClients {
		activeClient := activeClients[k]
		maxReqsPerSecond := 0

		if activeClient.ClientType == entity.Ip {
			maxReqsPerSecond = r.Configs.IpMaxReqsPerSecond
		} else {
			if tokenConfig, ok := r.Configs.TokenConfigs[activeClient.ClientId]; ok {
				maxReqsPerSecond = tokenConfig
			}
		}

		if entry, ok := activeClients[k]; ok {
			entry.Limiter = getRateLimiter(maxReqsPerSecond)
			activeClients[k] = entry
		}
	}

	r.activeClients.mu.Lock()
	r.activeClients.clients = activeClients
	r.activeClients.mu.Unlock()
}

func (r *RateLimiter) saveActiveClients() {

	r.activeClients.mu.Lock()
	activeClients := make(map[string]entity.ActiveClient, len(r.activeClients.clients))

	for k, v := range r.activeClients.clients {
		activeClients[k] = v
	}
	r.activeClients.mu.Unlock()

	log.Println("Saving active clients...", activeClients)
	err := r.Repository.SaveActiveClients(activeClients)
	if err != nil {
		panic(err)
	}
}

func (r *RateLimiter) addActiveClient(client entity.ActiveClient) {

	log.Println("Adding active client", client)

	r.activeClients.mu.Lock()
	r.activeClients.clients[client.ClientId] = client
	r.activeClients.mu.Unlock()

	r.saveActiveClients()
}

func (r *RateLimiter) removeActiveClient(client entity.ActiveClient) {

	log.Println("Removing active client", client)

	r.activeClients.mu.Lock()
	delete(r.activeClients.clients, client.ClientId)
	r.activeClients.mu.Unlock()

	r.saveActiveClients()
}

func (r *RateLimiter) updateActiveClient(client entity.ActiveClient) {

	log.Println("Updating active client", client)

	r.activeClients.mu.Lock()
	r.activeClients.clients[client.ClientId] = client
	r.activeClients.mu.Unlock()

	r.saveActiveClients()
}

func (r *RateLimiter) unblockActiveClient(key string) {

	log.Println("Unblocking active client", key)

	r.activeClients.mu.Lock()
	client := r.activeClients.clients[key]
	client.Blocked = false
	client.BlockedUntil = time.Time{}
	r.activeClients.clients[key] = client
	r.activeClients.mu.Unlock()

	r.saveActiveClients()
}

func (r *RateLimiter) Allow(ipAddr string, apiKeyHeader string) bool {

	if apiKeyHeader != "" {

		tokenMaxReqsPerSecond, ok := r.Configs.TokenConfigs[apiKeyHeader]
		if ok {

			log.Println("tokenMaxReqsPerSecond", tokenMaxReqsPerSecond)
			return r.verifyClientAllowed(apiKeyHeader, entity.Token, tokenMaxReqsPerSecond)
		}
	}

	ipMaxReqsPerSecond := r.Configs.IpMaxReqsPerSecond
	log.Println("ipMaxReqsPerSecond", ipMaxReqsPerSecond)
	return r.verifyClientAllowed(ipAddr, entity.Ip, ipMaxReqsPerSecond)
}

func (r *RateLimiter) verifyClientAllowed(id string, clientType entity.ClientType, maxReqsPerSecond int) bool {
	log.Println("verifyClientAllowed", id)

	r.activeClients.mu.Lock()
	activeClient, exists := r.activeClients.clients[id]
	r.activeClients.mu.Unlock()

	if !exists {
		activeClient = createActiveClient(id, clientType, maxReqsPerSecond)
		r.addActiveClient(activeClient)
		log.Println("Active clients: ", r.activeClients.clients)
		allow := activeClient.Limiter.Allow()
		log.Println("Allow", allow)
		return allow
	}

	log.Println("Existing active client", activeClient)

	activeClient.LastSeen = time.Now()

	if activeClient.Blocked {
		log.Println("Client is blocked until", activeClient.BlockedUntil)
		r.updateActiveClient(activeClient)
		return false
	}

	allow := activeClient.Limiter.Allow()

	if !allow {
		activeClient.Blocked = true
		activeClient.BlockedUntil = time.Now().Add(r.Configs.BlockingDuration)
		log.Printf("Blocking client %s until %s\n", activeClient.ClientId, activeClient.BlockedUntil)
	}

	r.updateActiveClient(activeClient)

	log.Println("Allow", allow)
	return allow
}

func createActiveClient(id string, clientType entity.ClientType, maxReqsPerSeconds int) entity.ActiveClient {
	return entity.ActiveClient{
		ClientId:     id,
		LastSeen:     time.Now(),
		ClientType:   clientType,
		BlockedUntil: time.Time{},
		Blocked:      false,
		Limiter:      getRateLimiter(maxReqsPerSeconds),
	}
}

func getRateLimiter(maxReqsPerSecond int) *rate.Limiter {
	return rate.NewLimiter(rate.Limit(maxReqsPerSecond), maxReqsPerSecond)
}
