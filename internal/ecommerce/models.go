package ecommerce

import "time"

// Order represents a customer order from any ecommerce platform
type Order struct {
	ID              string         `json:"id"`
	IncrementID     string         `json:"increment_id"`     // Display order number
	CustomerEmail   string         `json:"customer_email"`
	CustomerName    string         `json:"customer_name"`
	Status          string         `json:"status"`
	State           string         `json:"state"`
	Items           []OrderItem    `json:"items"`
	Subtotal        float64        `json:"subtotal"`
	GrandTotal      float64        `json:"grand_total"`
	TotalPaid       float64        `json:"total_paid"`
	TotalRefunded   float64        `json:"total_refunded"`
	ShippingAmount  float64        `json:"shipping_amount"`
	Currency        string         `json:"currency"`
	PaymentMethod   string         `json:"payment_method"`
	ShippingMethod  string         `json:"shipping_method"`
	ShippingAddress *Address       `json:"shipping_address"`
	BillingAddress  *Address       `json:"billing_address"`
	Shipments       []Shipment     `json:"shipments"`
	StatusHistory   []StatusEntry  `json:"status_history"`
	CreatedAt       time.Time      `json:"created_at"`
}

// StatusEntry represents a status change or note in the order history
type StatusEntry struct {
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

// OrderItem represents a line item in an order
type OrderItem struct {
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	Qty         int     `json:"qty"`
	QtyShipped  int     `json:"qty_shipped"`
	QtyRefunded int     `json:"qty_refunded"`
	Price       float64 `json:"price"`
	RowTotal    float64 `json:"row_total"`
}

// Shipment represents a shipment for an order
type Shipment struct {
	ID             string    `json:"id"`
	TrackingNumber string    `json:"tracking_number"`
	Carrier        string    `json:"carrier"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// Address represents a customer address
type Address struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Street    string `json:"street"`
	City      string `json:"city"`
	Region    string `json:"region"`
	PostCode  string `json:"postcode"`
	Country   string `json:"country"`
	Telephone string `json:"telephone"`
}

// Customer represents a customer profile
type Customer struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Telephone string    `json:"telephone"`
	CreatedAt time.Time `json:"created_at"`
	Orders    []Order   `json:"orders,omitempty"`
}

// EcommerceContext contains all ecommerce data for AI context
type EcommerceContext struct {
	Customer      *Customer `json:"customer,omitempty"`
	RecentOrders  []Order   `json:"recent_orders,omitempty"`
	MatchedOrders []*Order  `json:"matched_orders,omitempty"` // Orders mentioned in conversation
	Warnings      []string  `json:"warnings,omitempty"`
}

// ProviderConfig contains the configuration for an ecommerce provider
type ProviderConfig struct {
	Type         string            `json:"type"`          // "magento1", "magento2", "shopify"
	BaseURL      string            `json:"base_url"`
	ClientID     string            `json:"client_id"`
	ClientSecret string            `json:"client_secret"` // Encrypted in database
	ExtraConfig  map[string]string `json:"extra_config"`  // Provider-specific settings
}
