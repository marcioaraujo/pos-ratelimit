package database

import (
	"context"
	"encoding/json"
	"log"

	"github.com/go-redis/redis/v8"
	entity "github.com/marcioaraujo/pos-ratelimit/internal/entity"
)

type RateLimiterRedisRepository struct {
	ctx    context.Context
	client *redis.Client
}

func NewRateLimiterRedisRepository(ctx context.Context, client *redis.Client) *RateLimiterRedisRepository {
	return &RateLimiterRedisRepository{ctx: ctx, client: client}
}

func (r *RateLimiterRedisRepository) SaveActiveClients(clients map[string]entity.ActiveClient) error {

	value, err := json.Marshal(clients)
	if err != nil {
		log.Println("Error marshalling clients to JSON", err)
		return err
	}

	for _, client := range clients {
		err = r.client.Set(r.ctx, client.ClientId, value, 0).Err()
		if err != nil {
			log.Println("Error saving active client to Redis", err)
		}
	}

	return err
}

func (r *RateLimiterRedisRepository) GetActiveClients() (map[string]entity.ActiveClient, error) {

	activeClients := make(map[string]entity.ActiveClient, 0)

	keys := make([]string, 0)

	iter := r.client.Scan(r.ctx, 0, "", 0).Iterator()
	for iter.Next(r.ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		panic(err)
	}

	for _, key := range keys {
		value, err := r.client.Get(r.ctx, key).Result()
		if err == redis.Nil {
			log.Println("Error getting active client from Redis. Key does not exist", key)
			continue
		} else if err != nil {
			log.Println("Error getting active client from Redis", err)
			continue
		}

		var activeClient map[string]entity.ActiveClient
		err = json.Unmarshal([]byte(value), &activeClient)
		if err != nil {
			log.Println("Error unmarshalling active client from Redis", err)
			continue
		}

		activeClients[key] = activeClient[key]
	}

	return activeClients, nil
}
