package entity

import (
	"time"

	"golang.org/x/time/rate"
)

type ClientType uint8

const (
	Ip ClientType = iota
	Token
)

type ActiveClient struct {
	ClientId     string        `json:"clientId"`
	LastSeen     time.Time     `json:"lastSeen"`
	ClientType   ClientType    `json:"clientType"`
	BlockedUntil time.Time     `json:"blockedUntil"`
	Blocked      bool          `json:"blocked"`
	Limiter      *rate.Limiter `json:"-"`
}
