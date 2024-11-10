package ratelimiter

import (
	"context"
	"database/sql"
	"testing"
	"time"

	entity "github.com/marcioaraujo/pos-ratelimit/internal/entity"
	db "github.com/marcioaraujo/pos-ratelimit/internal/infra/database"
	"github.com/stretchr/testify/suite"
)

type RateLimiterTestSuite struct {
	suite.Suite
	Ctx        context.Context
	Cancel     context.CancelFunc
	Db         *sql.DB
	Repository db.RateLimiterRepository
}

func (suite *RateLimiterTestSuite) SetupSuite() {
	suite.Ctx, suite.Cancel = context.WithCancel(context.Background())
	client, err := sql.Open("sqlite3", ":memory:")
	suite.NoError(err)
	client.Exec("CREATE TABLE active_client (ClientId TEXT NOT NULL, LastSeen DATETIME NOT NULL, ClientType INTEGER NOT NULL, BlockedUntil DATETIME, Blocked BOOLEAN NOT NULL)")
	suite.Db = client
}

func (suite *RateLimiterTestSuite) SetupTest() {
	suite.Db.Ping()
	suite.Repository = db.NewRateLimiterSQLiteRepository(suite.Ctx, suite.Db)
}

func (suite *RateLimiterTestSuite) TearDownTest() {
	suite.Cancel()
	suite.Ctx, suite.Cancel = context.WithCancel(context.Background())
	suite.Db.Exec("DELETE FROM active_client")
}

func (suite *RateLimiterTestSuite) TearDownSuite() {
	suite.Db.Close()
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(RateLimiterTestSuite))
}

func (suite *RateLimiterTestSuite) TestGivenIpAddress_WhenAllow_ThenShouldSaveIpClientTypeActiveClient() {

	rateLimiter := NewRateLimiter(suite.Ctx, RateLimiterConfigs{}, suite.Repository)

	rateLimiter.Allow("127.0.0.1", "")

	activeClients, err := suite.Repository.GetActiveClients()

	suite.NoError(err)
	suite.NotEmpty(activeClients)
	suite.Equal(1, len(activeClients))
	suite.Equal("127.0.0.1", activeClients["127.0.0.1"].ClientId)
	suite.Equal(entity.Ip, activeClients["127.0.0.1"].ClientType)
}

func (suite *RateLimiterTestSuite) TestGivenIpAddressAndToken_WhenAllow_ThenShouldSaveTokenClientTypeActiveClient() {

	configs := RateLimiterConfigs{
		IpMaxReqsPerSecond: 1,
		BlockingDuration:   5 * time.Second,
		TokenConfigs: map[string]int{
			"abcd12": 1,
		},
	}

	rateLimiter := NewRateLimiter(suite.Ctx, configs, suite.Repository)

	rateLimiter.Allow("127.0.0.1", "abcd12")

	activeClients, err := suite.Repository.GetActiveClients()

	suite.NoError(err)
	suite.NotEmpty(activeClients)
	suite.Equal(1, len(activeClients))
	suite.Equal("abcd12", activeClients["abcd12"].ClientId)
	suite.Equal(entity.Token, activeClients["abcd12"].ClientType)
}

func (suite *RateLimiterTestSuite) TestGivenExistingActiveClients_WhenCreateRateLimiter_ThenShouldLoadActiveClients() {

	clients := map[string]entity.ActiveClient{
		"127.0.0.1": {
			ClientId:   "127.0.0.1",
			LastSeen:   time.Now(),
			ClientType: entity.Ip,
		},
		"abcd12": {
			ClientId:   "abcd12",
			LastSeen:   time.Now(),
			ClientType: entity.Token,
		},
	}

	suite.Repository.SaveActiveClients(clients)

	NewRateLimiter(suite.Ctx, RateLimiterConfigs{}, suite.Repository)

	activeClients, err := suite.Repository.GetActiveClients()

	suite.NoError(err)
	suite.NotEmpty(activeClients)
	suite.Equal(2, len(activeClients))
	suite.Equal("127.0.0.1", activeClients["127.0.0.1"].ClientId)
	suite.Equal(entity.Ip, activeClients["127.0.0.1"].ClientType)
	suite.Equal("abcd12", activeClients["abcd12"].ClientId)
	suite.Equal(entity.Token, activeClients["abcd12"].ClientType)
}

func (suite *RateLimiterTestSuite) TestGivenExistingActiveClientsWithBlockingExpired_WhenCreateRateLimiter_ThenShouldLoadActiveClientsAndUnblockExpiredClientBlocks() {

	clients := map[string]entity.ActiveClient{
		"127.0.0.1": {
			ClientId:     "127.0.0.1",
			LastSeen:     time.Now(),
			ClientType:   entity.Ip,
			Blocked:      true,
			BlockedUntil: time.Now().Add(-1 * time.Minute),
		},
		"abcd12": {
			ClientId:     "abcd12",
			LastSeen:     time.Now(),
			ClientType:   entity.Token,
			Blocked:      true,
			BlockedUntil: time.Now().Add(-1 * time.Minute),
		},
	}

	suite.Repository.SaveActiveClients(clients)

	NewRateLimiter(suite.Ctx, RateLimiterConfigs{}, suite.Repository)

	time.Sleep(2 * time.Second)

	activeClients, err := suite.Repository.GetActiveClients()

	suite.NoError(err)
	suite.NotEmpty(activeClients)
	suite.Equal(2, len(activeClients))
	suite.Equal("127.0.0.1", activeClients["127.0.0.1"].ClientId)
	suite.Equal(entity.Ip, activeClients["127.0.0.1"].ClientType)
	suite.Equal(false, activeClients["127.0.0.1"].Blocked)
	suite.Equal("abcd12", activeClients["abcd12"].ClientId)
	suite.Equal(entity.Token, activeClients["abcd12"].ClientType)
	suite.Equal(false, activeClients["abcd12"].Blocked)
}

func (suite *RateLimiterTestSuite) TestGivenIpAddress_WhenAllowCalledTwiceWithinASecondAndLimitIsOne_ThenShouldSaveBlockedClientAndReturnFalse() {

	configs := RateLimiterConfigs{
		IpMaxReqsPerSecond: 1,
		BlockingDuration:   30 * time.Second,
		TokenConfigs: map[string]int{
			"abcd12": 1,
		},
	}

	rateLimiter := NewRateLimiter(suite.Ctx, configs, suite.Repository)

	response := rateLimiter.Allow("127.0.0.1", "")
	suite.True(response)

	time.Sleep(100 * time.Millisecond)

	response = rateLimiter.Allow("127.0.0.1", "")

	activeClients, err := suite.Repository.GetActiveClients()

	suite.NoError(err)
	suite.NotEmpty(activeClients)
	suite.Equal(1, len(activeClients))
	suite.Equal("127.0.0.1", activeClients["127.0.0.1"].ClientId)
	suite.Equal(entity.Ip, activeClients["127.0.0.1"].ClientType)
	suite.Equal(true, activeClients["127.0.0.1"].Blocked)
	suite.False(response)
}

func (suite *RateLimiterTestSuite) TestGivenIpAddress_WhenClientBlocked_ThenShouldUnblockOnlyAfterBlockingTime() {

	configs := RateLimiterConfigs{
		IpMaxReqsPerSecond: 1,
		BlockingDuration:   3 * time.Second,
		TokenConfigs: map[string]int{
			"abcd12": 1,
		},
	}

	rateLimiter := NewRateLimiter(suite.Ctx, configs, suite.Repository)

	response := rateLimiter.Allow("127.0.0.1", "")
	suite.True(response)

	time.Sleep(100 * time.Millisecond)

	response = rateLimiter.Allow("127.0.0.1", "")

	activeClients, err := suite.Repository.GetActiveClients()

	suite.NoError(err)
	suite.NotEmpty(activeClients)
	suite.Equal(1, len(activeClients))
	suite.Equal("127.0.0.1", activeClients["127.0.0.1"].ClientId)
	suite.Equal(entity.Ip, activeClients["127.0.0.1"].ClientType)
	suite.True(activeClients["127.0.0.1"].Blocked)
	suite.False(response)

	time.Sleep(6 * time.Second)

	response = rateLimiter.Allow("127.0.0.1", "")
	suite.True(response)

	activeClients, err = suite.Repository.GetActiveClients()

	suite.NoError(err)
	suite.NotEmpty(activeClients)
	suite.Equal(1, len(activeClients))
	suite.Equal("127.0.0.1", activeClients["127.0.0.1"].ClientId)
	suite.Equal(entity.Ip, activeClients["127.0.0.1"].ClientType)
	suite.False(activeClients["127.0.0.1"].Blocked)
}

func (suite *RateLimiterTestSuite) TestGivenIpAddressAndToken_WhenClientBlocked_ThenShouldUseTokenConfigAndUnblockOnlyAfterBlockingTime() {

	configs := RateLimiterConfigs{
		IpMaxReqsPerSecond: 1,
		BlockingDuration:   3 * time.Second,
		TokenConfigs: map[string]int{
			"abcd12": 2,
		},
	}

	rateLimiter := NewRateLimiter(suite.Ctx, configs, suite.Repository)

	response := rateLimiter.Allow("127.0.0.1", "abcd12")
	suite.True(response)

	time.Sleep(100 * time.Millisecond)

	response = rateLimiter.Allow("127.0.0.1", "abcd12")
	suite.True(response)

	time.Sleep(100 * time.Millisecond)

	response = rateLimiter.Allow("127.0.0.1", "abcd12")
	suite.False(response)

	activeClients, err := suite.Repository.GetActiveClients()

	suite.NoError(err)
	suite.NotEmpty(activeClients)
	suite.Equal(1, len(activeClients))
	suite.Equal("abcd12", activeClients["abcd12"].ClientId)
	suite.Equal(entity.Token, activeClients["abcd12"].ClientType)
	suite.True(activeClients["abcd12"].Blocked)
	suite.False(response)

	time.Sleep(6 * time.Second)

	response = rateLimiter.Allow("127.0.0.1", "abcd12")
	suite.True(response)

	activeClients, err = suite.Repository.GetActiveClients()

	suite.NoError(err)
	suite.NotEmpty(activeClients)
	suite.Equal(1, len(activeClients))
	suite.Equal("abcd12", activeClients["abcd12"].ClientId)
	suite.Equal(entity.Token, activeClients["abcd12"].ClientType)
	suite.False(activeClients["abcd12"].Blocked)
}
