package client

import (
	"context"

	"github.com/stretchr/testify/mock"

	"extend-challenge-common/pkg/domain"
)

// MockRewardClient is a mock implementation of RewardClient for testing.
// It uses testify/mock to allow test assertions on method calls.
type MockRewardClient struct {
	mock.Mock
}

// GrantItemReward mocks granting an item reward.
func (m *MockRewardClient) GrantItemReward(ctx context.Context, namespace, userID, itemID string, quantity int) error {
	args := m.Called(ctx, namespace, userID, itemID, quantity)
	return args.Error(0)
}

// GrantWalletReward mocks granting a wallet reward.
func (m *MockRewardClient) GrantWalletReward(ctx context.Context, namespace, userID, currencyCode string, amount int) error {
	args := m.Called(ctx, namespace, userID, currencyCode, amount)
	return args.Error(0)
}

// GrantReward mocks the convenience method for granting rewards.
func (m *MockRewardClient) GrantReward(ctx context.Context, namespace, userID string, reward domain.Reward) error {
	args := m.Called(ctx, namespace, userID, reward)
	return args.Error(0)
}

// NewMockRewardClient creates a new mock reward client.
func NewMockRewardClient() *MockRewardClient {
	return &MockRewardClient{}
}
