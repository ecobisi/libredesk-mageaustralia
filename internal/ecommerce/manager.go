package ecommerce

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/zerodha/logf"
)

// Manager handles ecommerce provider operations with multi-stage context gathering
type Manager struct {
	provider Provider
	lo       logf.Logger
}

// NewManager creates a new ecommerce manager
func NewManager(provider Provider, lo logf.Logger) *Manager {
	return &Manager{provider: provider, lo: lo}
}

// IsConfigured returns true if a provider is configured
func (m *Manager) IsConfigured() bool {
	return m.provider != nil
}

// GatherFullContext performs multi-stage context gathering for AI prompt
// Stage 1: Fetch customer + recent orders by email
// Stage 2: Scan all provided messages for order numbers
// Stage 3: Fetch full details for mentioned orders
func (m *Manager) GatherFullContext(ctx context.Context, email string, messages []string, maxOrders int) (*EcommerceContext, error) {
	if m.provider == nil {
		return nil, nil
	}

	result := &EcommerceContext{}

	// Stage 1: Fetch customer and recent orders
	customer, err := m.provider.GetCustomerByEmail(ctx, email)
	if err != nil && err != ErrNotFound {
		m.lo.Warn("failed to get customer", "email", email, "error", err)
		result.Warnings = append(result.Warnings, fmt.Sprintf("Customer lookup failed: %v", err))
	} else if err == nil {
		result.Customer = customer
	}

	orders, err := m.provider.GetOrdersByEmail(ctx, email, maxOrders)
	if err != nil && err != ErrNotFound {
		m.lo.Warn("failed to get orders", "email", email, "error", err)
		result.Warnings = append(result.Warnings, fmt.Sprintf("Recent orders lookup failed: %v", err))
	} else {
		result.RecentOrders = orders
	}

	// Stage 2: Scan ALL messages for order numbers
	m.lo.Info("scanning messages for order numbers", "message_count", len(messages))
	var foundOrderNumbers []string
	for _, msg := range messages {
		nums := extractAllOrderNumbers(msg)
		if len(nums) > 0 {
			m.lo.Info("found order numbers in message", "numbers", nums)
		}
		foundOrderNumbers = append(foundOrderNumbers, nums...)
	}
	m.lo.Info("order number scan complete", "found", foundOrderNumbers)

	// Deduplicate
	seen := make(map[string]bool)
	var uniqueOrders []string
	for _, num := range foundOrderNumbers {
		if !seen[num] {
			seen[num] = true
			uniqueOrders = append(uniqueOrders, num)
		}
	}

	// Stage 3: Fetch full details for mentioned orders (limit to first 3)
	m.lo.Info("Stage 3: fetching mentioned orders", "unique_orders", uniqueOrders)
	for i, orderNum := range uniqueOrders {
		if i >= 3 {
			break
		}
		// Skip if already in recent orders
		alreadyHave := false
		for _, ro := range result.RecentOrders {
			if ro.IncrementID == orderNum {
				// Promote to matched order with full data
				o := ro
				result.MatchedOrders = append(result.MatchedOrders, &o)
				alreadyHave = true
				break
			}
		}
		if alreadyHave {
			continue
		}
		order, err := m.provider.GetOrderByNumber(ctx, orderNum)
		if err == nil {
			result.MatchedOrders = append(result.MatchedOrders, order)
			m.lo.Debug("found order in conversation", "order_number", orderNum)
		} else if err != ErrNotFound {
			m.lo.Warn("failed to lookup order", "order_number", orderNum, "error", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Order #%s lookup failed: %v", orderNum, err))
		}
	}

	// Deduplicate warnings — same auth/network failure cascades across stages
	if len(result.Warnings) > 0 {
		seen := make(map[string]bool, len(result.Warnings))
		deduped := result.Warnings[:0]
		for _, w := range result.Warnings {
			if !seen[w] {
				seen[w] = true
				deduped = append(deduped, w)
			}
		}
		result.Warnings = deduped
	}

	return result, nil
}

// FormatContextForPrompt formats ecommerce context as text for AI prompt
func (m *Manager) FormatContextForPrompt(eCtx *EcommerceContext) string {
	if eCtx == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Customer Ecommerce Data\n\n")

	if eCtx.Customer != nil {
		sb.WriteString(fmt.Sprintf("**Customer:** %s %s (%s)\n",
			eCtx.Customer.FirstName, eCtx.Customer.LastName, eCtx.Customer.Email))
		if eCtx.Customer.Telephone != "" {
			sb.WriteString(fmt.Sprintf("**Phone:** %s\n", eCtx.Customer.Telephone))
		}
		if !eCtx.Customer.CreatedAt.IsZero() {
			sb.WriteString(fmt.Sprintf("**Customer since:** %s\n", eCtx.Customer.CreatedAt.Format("2006-01-02")))
		}
	}

	// Show matched orders (mentioned in conversation) with FULL details
	if len(eCtx.MatchedOrders) > 0 {
		sb.WriteString("\n### Orders Mentioned in Conversation\n")
		for _, order := range eCtx.MatchedOrders {
			sb.WriteString(formatOrderFull(order))
			sb.WriteString("\n")
		}
	}

	// Show recent orders as summary only
	if len(eCtx.RecentOrders) > 0 {
		sb.WriteString("\n### Recent Orders (Summary)\n")
		for _, order := range eCtx.RecentOrders {
			// Skip if already shown in matched orders
			alreadyShown := false
			for _, matched := range eCtx.MatchedOrders {
				if matched.IncrementID == order.IncrementID {
					alreadyShown = true
					break
				}
			}
			if !alreadyShown {
				sb.WriteString(formatOrderSummary(&order))
			}
		}
	}

	return sb.String()
}

func formatOrderFull(o *Order) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n**Order #%s**\n", o.IncrementID))
	sb.WriteString(fmt.Sprintf("- Status: %s\n", o.Status))
	sb.WriteString(fmt.Sprintf("- Date: %s\n", o.CreatedAt.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("- Total: $%.2f %s\n", o.GrandTotal, o.Currency))

	if o.TotalPaid > 0 {
		sb.WriteString(fmt.Sprintf("- Paid: $%.2f\n", o.TotalPaid))
	}
	if o.TotalRefunded > 0 {
		sb.WriteString(fmt.Sprintf("- Refunded: $%.2f\n", o.TotalRefunded))
	}
	if o.PaymentMethod != "" {
		sb.WriteString(fmt.Sprintf("- Payment: %s\n", o.PaymentMethod))
	}
	if o.ShippingMethod != "" {
		sb.WriteString(fmt.Sprintf("- Shipping: %s\n", o.ShippingMethod))
	}
	if o.ShippingAddress != nil {
		parts := []string{}
		if o.ShippingAddress.City != "" {
			parts = append(parts, o.ShippingAddress.City)
		}
		if o.ShippingAddress.Region != "" {
			parts = append(parts, o.ShippingAddress.Region)
		}
		if o.ShippingAddress.PostCode != "" {
			parts = append(parts, o.ShippingAddress.PostCode)
		}
		if o.ShippingAddress.Country != "" {
			parts = append(parts, o.ShippingAddress.Country)
		}
		if len(parts) > 0 {
			sb.WriteString(fmt.Sprintf("- Shipping to: %s\n", strings.Join(parts, ", ")))
		}
	}

	if len(o.Items) > 0 {
		sb.WriteString("- Items:\n")
		for _, item := range o.Items {
			line := fmt.Sprintf("  - %s (SKU: %s) x%d @ $%.2f = $%.2f",
				item.Name, item.SKU, item.Qty, item.Price, item.RowTotal)
			if item.QtyRefunded > 0 {
				line += fmt.Sprintf(" [REFUNDED x%d]", item.QtyRefunded)
			}
			if item.QtyShipped > 0 {
				line += fmt.Sprintf(" [SHIPPED x%d]", item.QtyShipped)
			}
			sb.WriteString(line + "\n")
		}
	}

	if len(o.Shipments) > 0 {
		sb.WriteString("- Shipments:\n")
		for _, ship := range o.Shipments {
			trackURL := trackingURL(ship.Carrier, ship.TrackingNumber)
			if trackURL != "" {
				sb.WriteString(fmt.Sprintf("  - %s Tracking: %s ( %s )\n", ship.Carrier, ship.TrackingNumber, trackURL))
			} else {
				sb.WriteString(fmt.Sprintf("  - %s Tracking: %s\n", ship.Carrier, ship.TrackingNumber))
			}
		}
	}

	if o.ShippingAddress != nil {
		sb.WriteString(fmt.Sprintf("- Ship to: %s %s, %s, %s %s %s\n",
			o.ShippingAddress.FirstName, o.ShippingAddress.LastName,
			o.ShippingAddress.Street,
			o.ShippingAddress.City, o.ShippingAddress.Region, o.ShippingAddress.PostCode))
	}

	// Status history - include all notes (most recent first for relevance)
	if len(o.StatusHistory) > 0 {
		sb.WriteString("- Order History:\n")
		for _, entry := range o.StatusHistory {
			if entry.Note != "" {
				sb.WriteString(fmt.Sprintf("  - [%s] %s\n", entry.CreatedAt, entry.Note))
			}
		}
	}

	return sb.String()
}

func formatOrderSummary(o *Order) string {
	summary := fmt.Sprintf("- #%s | %s | $%.2f %s | %s",
		o.IncrementID, o.Status, o.GrandTotal, o.Currency, o.CreatedAt.Format("2006-01-02"))
	if o.TotalRefunded > 0 {
		summary += fmt.Sprintf(" | Refunded: $%.2f", o.TotalRefunded)
	}
	return summary + "\n"
}

// trackingURL returns the carrier tracking URL for a given tracking number.
func trackingURL(carrier, trackingNumber string) string {
	c := strings.ToLower(carrier)
	switch {
	case strings.Contains(c, "australia post") || strings.Contains(c, "auspost") || strings.Contains(c, "eparcel"):
		return "https://auspost.com.au/mypost/track/details/" + trackingNumber
	case strings.Contains(c, "couriers please") || strings.Contains(c, "couriersplease"):
		return "https://www.couriersplease.com.au/tools-track/no/" + trackingNumber
	case strings.Contains(c, "team global") || strings.Contains(c, "tge") || strings.Contains(c, "toll"):
		return "https://www.myteamge.com/?externalSearchQuery=" + trackingNumber
	default:
		return ""
	}
}

// Order number patterns for Magento-style IDs (100xxxxxx)
var (
	orderPrefixRegex     = regexp.MustCompile(`(?i)(?:order|#|number)[:\s#]*(\d{9,12})`)
	standaloneOrderRegex = regexp.MustCompile(`\b(1\d{8,11})\b`)
)

func extractAllOrderNumbers(text string) []string {
	var results []string

	// First try prefixed patterns (higher confidence)
	matches := orderPrefixRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 {
			results = append(results, match[1])
		}
	}

	// Then try standalone numbers
	matches = standaloneOrderRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 {
			results = append(results, match[1])
		}
	}

	return results
}

// GetOrderByNumber looks up an order by its display number
func (m *Manager) GetOrderByNumber(ctx context.Context, orderNumber string) (*Order, error) {
	if m.provider == nil {
		return nil, fmt.Errorf("no provider configured")
	}
	return m.provider.GetOrderByNumber(ctx, orderNumber)
}
