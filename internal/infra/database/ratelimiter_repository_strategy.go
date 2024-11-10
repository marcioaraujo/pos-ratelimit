package database

import (
	"context"
	"database/sql"
	"log"

	"github.com/go-redis/redis/v8"
	config "github.com/marcioaraujo/pos-ratelimit/configs"
)

func RateLimiterRepositoryStrategy(ctx context.Context, configs config.PersistenceConfigs, database string) RateLimiterRepository {
	var repository RateLimiterRepository
	switch database {
	case "redis":
		repository = NewRateLimiterRedisRepository(ctx, getRedisClient(ctx, configs))
	case "sqlite":
		repository = NewRateLimiterSQLiteRepository(ctx, getSQLiteClient())
	default:
		repository = NewRateLimiterRedisRepository(ctx, getRedisClient(ctx, configs))
	}
	return repository
}

func getRedisClient(ctx context.Context, configs config.PersistenceConfigs) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     configs.Redis.Addr,
		Password: configs.Redis.Password,
		DB:       configs.Redis.Db,
	})

	pong, err := redisClient.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}
	log.Println("Redis connected", pong)

	return redisClient
}

func getSQLiteClient() *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	return db
}
