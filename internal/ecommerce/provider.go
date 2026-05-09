package ecommerce

import (
	"context"
	"fmt"
)

// Provider defines the interface for ecommerce platform integrations
type Provider interface {
	// Name returns the provider identifier (e.g., "magento1", "magento2", "shopify")
	Name() string

	// GetCustomerByEmail looks up a customer by email
	GetCustomerByEmail(ctx context.Context, email string) (*Customer, error)

	// GetOrdersByEmail returns recent orders for an email address
	GetOrdersByEmail(ctx context.Context, email string, limit int) ([]Order, error)

	// GetOrderByNumber looks up an order by its display number (increment_id)
	GetOrderByNumber(ctx context.Context, orderNumber string) (*Order, error)

	// GetOrderByID looks up an order by internal ID
	GetOrderByID(ctx context.Context, orderID string) (*Order, error)

	// TestConnection verifies the provider configuration is valid
	TestConnection(ctx context.Context) error
}

// ErrNotFound is returned when a resource doesn't exist
var ErrNotFound = fmt.Errorf("not found")

// ErrUnauthorized is returned when authentication fails
var ErrUnauthorized = fmt.Errorf("unauthorized")
