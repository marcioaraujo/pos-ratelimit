package database

import entity "github.com/marcioaraujo/pos-ratelimit/internal/entity"

type RateLimiterRepository interface {
	GetActiveClients() (map[string]entity.ActiveClient, error)
	SaveActiveClients(clients map[string]entity.ActiveClient) error
}
