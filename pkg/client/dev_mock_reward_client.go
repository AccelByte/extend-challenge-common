package client

import (
	"context"
	"log"

	"github.com/AccelByte/extend-challenge-common/pkg/domain"
)

// DevMockRewardClient is a simple mock implementation for local development.
// Unlike MockRewardClient (testify/mock), this doesn't require explicit setup
// and always succeeds with logged output.
//
// Use this for local development when REWARD_CLIENT_MODE=mock.
// For tests, use MockRewardClient instead.
type DevMockRewardClient struct{}

// GrantItemReward logs the reward grant and returns success.
func (d *DevMockRewardClient) GrantItemReward(ctx context.Context, namespace, userID, itemID string, quantity int) error {
	log.Printf("[DevMock] GrantItemReward: namespace=%s, userID=%s, itemID=%s, quantity=%d",
		namespace, userID, itemID, quantity)
	return nil
}

// GrantWalletReward logs the reward grant and returns success.
func (d *DevMockRewardClient) GrantWalletReward(ctx context.Context, namespace, userID, currencyCode string, amount int) error {
	log.Printf("[DevMock] GrantWalletReward: namespace=%s, userID=%s, currencyCode=%s, amount=%d",
		namespace, userID, currencyCode, amount)
	return nil
}

// GrantReward logs the reward grant and returns success.
func (d *DevMockRewardClient) GrantReward(ctx context.Context, namespace, userID string, reward domain.Reward) error {
	log.Printf("[DevMock] GrantReward: namespace=%s, userID=%s, reward=%+v",
		namespace, userID, reward)

	// Delegate to specific methods for consistent logging
	switch reward.Type {
	case "ITEM":
		return d.GrantItemReward(ctx, namespace, userID, reward.RewardID, reward.Quantity)
	case "WALLET":
		return d.GrantWalletReward(ctx, namespace, userID, reward.RewardID, reward.Quantity)
	default:
		log.Printf("[DevMock] WARNING: Unknown reward type: %s (but still returning success)", reward.Type)
		return nil
	}
}

// NewDevMockRewardClient creates a new development mock reward client.
func NewDevMockRewardClient() *DevMockRewardClient {
	return &DevMockRewardClient{}
}
