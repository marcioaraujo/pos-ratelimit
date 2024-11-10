package web

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	configs "github.com/marcioaraujo/pos-ratelimit/configs"
	db "github.com/marcioaraujo/pos-ratelimit/internal/infra/database"
	rateLimiter "github.com/marcioaraujo/pos-ratelimit/internal/ratelimiter"
)

type RateLimiterMiddleware struct {
	RateLimiter *rateLimiter.RateLimiter
}

func NewRateLimiterMiddleware(
	Ctx context.Context,
	Configs configs.RateLimiterConfigs,
	Repository db.RateLimiterRepository,
) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{
		RateLimiter: rateLimiter.NewRateLimiter(
			Ctx,
			rateLimiter.RateLimiterConfigs{
				BlockingDuration:   Configs.BlockingDuration,
				IpMaxReqsPerSecond: Configs.IpMaxReqsPerSecond,
				TokenConfigs:       Configs.TokenConfigs},
			Repository),
	}
}

func (h *RateLimiterMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		apiKeyHeader := r.Header.Get("API_KEY")
		ipAddr, err := getIP(r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Println("ipAddr", ipAddr)
		log.Println("apiKeyHeader", apiKeyHeader)

		allow := h.RateLimiter.Allow(ipAddr, apiKeyHeader)

		if allow {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("you have reached the maximum number of requests or actions allowed within a certain time frame"))
		}
	})
}

func getIP(req *http.Request) (string, error) {
	ip, port, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		log.Printf("userip: %q is not IP:port\n", req.RemoteAddr)
		return "", err
	}

	userIP := net.ParseIP(ip)
	if userIP == nil {
		log.Printf("userip: %q is not IP:port\n", req.RemoteAddr)
		return "", fmt.Errorf("userip: %q is not IP:port", req.RemoteAddr)
	}

	// This will only be defined when site is accessed via non-anonymous proxy
	// and takes precedence over RemoteAddr
	// Header.Get is case-insensitive
	forward := req.Header.Get("X-Forwarded-For")

	log.Printf("IP: %s\n", ip)
	log.Printf("Port: %s\n", port)
	log.Printf("Forwarded for: %s\n", forward)

	return ip, nil
}
