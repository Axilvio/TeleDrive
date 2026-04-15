package payments

import (
	"context"
	"fmt"
)

type Store interface {
	CreatePendingPayment(ctx context.Context, userID int64, amount float64, provider, tariff string) (int64, error)
	ConfirmPaymentAndActivate(ctx context.Context, paymentID int64) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) CreateSubscriptionPayment(ctx context.Context, userID int64, tariff string) (int64, float64, error) {
	amount, err := tariffPrice(tariff)
	if err != nil {
		return 0, 0, err
	}
	paymentID, err := s.store.CreatePendingPayment(ctx, userID, amount, "telegram_stars", tariff)
	if err != nil {
		return 0, 0, err
	}
	return paymentID, amount, nil
}

func (s *Service) ConfirmPayment(ctx context.Context, paymentID int64) error {
	return s.store.ConfirmPaymentAndActivate(ctx, paymentID)
}

func tariffPrice(tariff string) (float64, error) {
	switch tariff {
	case "basic":
		return 299, nil
	case "standard":
		return 699, nil
	case "pro":
		return 1490, nil
	default:
		return 0, fmt.Errorf("unknown tariff: %s", tariff)
	}
}
