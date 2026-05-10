package magento1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/ecommerce"
	"github.com/zerodha/logf"
)

// Client implements the ecommerce.Provider interface for Maho Commerce
type Client struct {
	baseURL   string
	auth      *authClient
	http      *http.Client
	userAgent string
	lo        *logf.Logger
}

// userAgentString returns "libredesk/<version>" derived from the embedded
// build info so Maho-side access logs can identify libredesk traffic.
// Falls back to "libredesk/unknown" if build info isn't available (rare).
func userAgentString() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		v := info.Main.Version
		if v == "" || v == "(devel)" {
			// Look for a VCS revision setting populated by `go build`.
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" && s.Value != "" {
					rev := s.Value
					if len(rev) > 12 {
						rev = rev[:12]
					}
					return "libredesk/" + rev
				}
			}
			return "libredesk/devel"
		}
		return "libredesk/" + v
	}
	return "libredesk/unknown"
}

// New creates a new Maho Commerce client.
//
// Maho's API Platform v2 supports the OAuth2 client_credentials grant at
// /api/rest/v2/auth/token. ClientID and ClientSecret are the OAuth2 client
// credentials (ClientSecret is encrypted at rest by the settings layer).
func New(config ecommerce.ProviderConfig, lo *logf.Logger) (*Client, error) {
	if config.BaseURL == "" || config.ClientID == "" || config.ClientSecret == "" {
		return nil, fmt.Errorf("magento1: baseURL, clientID, and clientSecret are required")
	}
	ua := userAgentString()
	return &Client{
		baseURL:   config.BaseURL,
		auth:      newAuthClient(config.BaseURL, config.ClientID, config.ClientSecret, ua, lo),
		http:      &http.Client{Timeout: 60 * time.Second},
		userAgent: ua,
		lo:        lo,
	}, nil
}

func (c *Client) Name() string { return "magento1" }

// doRequest makes an authenticated request to the API
func (c *Client) doRequest(ctx context.Context, endpoint string, params url.Values) ([]byte, int, error) {
	token, err := c.auth.getToken()
	if err != nil {
		return nil, 0, err
	}

	u := c.baseURL + endpoint
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	c.lo.Debug("GET", "url", u)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	c.lo.Debug("response", "status", resp.StatusCode, "bytes", len(body))

	return body, resp.StatusCode, nil
}

// hydraCollection is the JSON-LD/Hydra collection wrapper
type hydraCollection struct {
	Member     json.RawMessage `json:"member"`
	TotalItems int             `json:"totalItems"`
}

// unwrapCollection handles both Hydra {"member":[...]} and plain arrays
func (c *Client) unwrapCollection(body []byte) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.RawMessage(trimmed), nil
	}
	var col hydraCollection
	if err := json.Unmarshal(trimmed, &col); err != nil {
		return nil, fmt.Errorf("decode hydra collection: %w", err)
	}
	if col.Member == nil {
		return json.RawMessage("[]"), nil
	}
	c.lo.Debug("hydra collection", "total_items", col.TotalItems)
	return col.Member, nil
}

// GetCustomerByEmail looks up a customer by email address
func (c *Client) GetCustomerByEmail(ctx context.Context, email string) (*ecommerce.Customer, error) {
	body, status, err := c.doRequest(ctx, "/api/rest/v2/customers", url.Values{"email": {email}})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", status)
	}

	members, err := c.unwrapCollection(body)
	if err != nil {
		return nil, fmt.Errorf("decode collection: %w", err)
	}

	var customers []mahoCustomer
	if err := json.Unmarshal(members, &customers); err != nil {
		return nil, fmt.Errorf("decode customers: %w", err)
	}
	if len(customers) == 0 {
		return nil, ecommerce.ErrNotFound
	}

	c0 := customers[0]
	c.lo.Debug("found customer", "first_name", c0.FirstName, "last_name", c0.LastName, "email", c0.Email)
	return c0.toEcommerce(), nil
}

// GetOrdersByEmail returns recent orders for an email address
func (c *Client) GetOrdersByEmail(ctx context.Context, email string, limit int) ([]ecommerce.Order, error) {
	params := url.Values{"email": {email}, "itemsPerPage": {fmt.Sprintf("%d", limit)}}
	body, status, err := c.doRequest(ctx, "/api/rest/v2/orders", params)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", status)
	}

	members, err := c.unwrapCollection(body)
	if err != nil {
		return nil, fmt.Errorf("decode collection: %w", err)
	}

	var orders []mahoOrder
	if err := json.Unmarshal(members, &orders); err != nil {
		return nil, fmt.Errorf("decode orders: %w", err)
	}

	c.lo.Debug("found orders", "count", len(orders), "email", email)

	result := make([]ecommerce.Order, len(orders))
	for i, o := range orders {
		result[i] = o.toEcommerce()
	}
	return result, nil
}

// GetOrderByNumber looks up an order by its display number (increment_id)
func (c *Client) GetOrderByNumber(ctx context.Context, orderNumber string) (*ecommerce.Order, error) {
	body, status, err := c.doRequest(ctx, "/api/rest/v2/orders", url.Values{"incrementId": {orderNumber}})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", status)
	}

	members, err := c.unwrapCollection(body)
	if err != nil {
		return nil, fmt.Errorf("decode collection: %w", err)
	}

	var orders []mahoOrder
	if err := json.Unmarshal(members, &orders); err != nil {
		return nil, fmt.Errorf("decode orders: %w", err)
	}
	if len(orders) == 0 {
		return nil, ecommerce.ErrNotFound
	}

	c.lo.Debug("found order", "increment_id", orders[0].IncrementID, "status", orders[0].Status, "history_entries", len(orders[0].StatusHistory))
	order := orders[0].toEcommerce()
	return &order, nil
}

// GetOrderByID looks up an order by internal ID
func (c *Client) GetOrderByID(ctx context.Context, orderID string) (*ecommerce.Order, error) {
	body, status, err := c.doRequest(ctx, "/api/rest/v2/orders/"+orderID, nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, ecommerce.ErrNotFound
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", status)
	}

	var order mahoOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}
	result := order.toEcommerce()
	return &result, nil
}

// TestConnection verifies the provider configuration is valid
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.auth.getToken()
	return err
}

// -------------------------------------------------------------------
// Maho API response types - matching exact field names from API
// -------------------------------------------------------------------

type mahoCustomer struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	FullName  string `json:"fullName"`
	Telephone string `json:"telephone"`
	CreatedAt string `json:"createdAt"`
}

func (m *mahoCustomer) toEcommerce() *ecommerce.Customer {
	created := parseTime(m.CreatedAt)
	return &ecommerce.Customer{
		ID:        fmt.Sprintf("%d", m.ID),
		Email:     m.Email,
		FirstName: m.FirstName,
		LastName:  m.LastName,
		Telephone: m.Telephone,
		CreatedAt: created,
	}
}

type mahoOrder struct {
	ID                  int                  `json:"id"`
	IncrementID         string               `json:"incrementId"`
	CustomerID          int                  `json:"customerId"`
	CustomerEmail       string               `json:"customerEmail"`
	CustomerFirstname   string               `json:"customerFirstname"`
	CustomerLastname    string               `json:"customerLastname"`
	Status              string               `json:"status"`
	State               string               `json:"state"`
	Currency            string               `json:"currency"`
	TotalItemCount      int                  `json:"totalItemCount"`
	TotalQtyOrdered     float64              `json:"totalQtyOrdered"`
	PaymentMethod       string               `json:"paymentMethod"`
	PaymentMethodTitle  string               `json:"paymentMethodTitle"`
	ShippingMethod      string               `json:"shippingMethod"`
	ShippingDescription string               `json:"shippingDescription"`
	CouponCode          string               `json:"couponCode"`
	Items               []mahoOrderItem      `json:"items"`
	Prices              mahoOrderPrices      `json:"prices"`
	ShippingAddress     *mahoAddress         `json:"shippingAddress"`
	BillingAddress      *mahoAddress         `json:"billingAddress"`
	Shipments           []mahoShipment       `json:"shipments"`
	StatusHistory       []mahoStatusEntry    `json:"statusHistory"`
	CreatedAt           string               `json:"createdAt"`
	UpdatedAt           string               `json:"updatedAt"`
}

type mahoStatusEntry struct {
	Note                string `json:"note"`
	CreatedAt           string `json:"createdAt"`
	IsCustomerNotified  bool   `json:"isCustomerNotified"`
	IsVisibleOnFront    bool   `json:"isVisibleOnFront"`
}

type mahoShipment struct {
	ID          int                `json:"id"`
	IncrementID string             `json:"incrementId"`
	TotalQty    float64            `json:"totalQty"`
	CreatedAt   string             `json:"createdAt"`
	Tracks      []mahoShipmentTrack `json:"tracks"`
}

type mahoShipmentTrack struct {
	ID          int    `json:"id"`
	Carrier     string `json:"carrier"`
	Title       string `json:"title"`
	TrackNumber string `json:"trackNumber"`
}

type mahoOrderItem struct {
	ID              int     `json:"id"`
	SKU             string  `json:"sku"`
	Name            string  `json:"name"`
	Qty             float64 `json:"qty"`
	QtyOrdered      float64 `json:"qtyOrdered"`
	QtyShipped      float64 `json:"qtyShipped"`
	QtyRefunded     float64 `json:"qtyRefunded"`
	QtyCanceled     float64 `json:"qtyCanceled"`
	Price           float64 `json:"price"`
	PriceInclTax    float64 `json:"priceInclTax"`
	RowTotal        float64 `json:"rowTotal"`
	RowTotalInclTax float64 `json:"rowTotalInclTax"`
	TaxAmount       float64 `json:"taxAmount"`
	ProductType     string  `json:"productType"`
}

type mahoOrderPrices struct {
	Subtotal       float64 `json:"subtotal"`
	GrandTotal     float64 `json:"grandTotal"`
	TotalPaid      float64 `json:"totalPaid"`
	TotalRefunded  float64 `json:"totalRefunded"`
	TotalDue       float64 `json:"totalDue"`
	ShippingAmount float64 `json:"shippingAmount"`
	TaxAmount      float64 `json:"taxAmount"`
	DiscountAmount float64 `json:"discountAmount"`
}

type mahoAddress struct {
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Street    []string `json:"street"`
	City      string   `json:"city"`
	Region    string   `json:"region"`
	Postcode  string   `json:"postcode"`
	CountryID string   `json:"countryId"`
	Telephone string   `json:"telephone"`
}

func (m *mahoOrder) toEcommerce() ecommerce.Order {
	created := parseTime(m.CreatedAt)

	items := make([]ecommerce.OrderItem, len(m.Items))
	for i, item := range m.Items {
		qty := int(item.QtyOrdered)
		if qty == 0 {
			qty = int(item.Qty)
		}
		items[i] = ecommerce.OrderItem{
			SKU:         item.SKU,
			Name:        item.Name,
			Qty:         qty,
			QtyShipped:  int(item.QtyShipped),
			QtyRefunded: int(item.QtyRefunded),
			Price:       item.Price,
			RowTotal:    item.RowTotal,
		}
	}

	cur := m.Currency
	if cur == "" {
		cur = "AUD"
	}

	payMethod := m.PaymentMethodTitle
	if payMethod == "" {
		payMethod = m.PaymentMethod
	}

	shipMethod := m.ShippingDescription
	if shipMethod == "" {
		shipMethod = m.ShippingMethod
	}

	// Convert status history
	history := make([]ecommerce.StatusEntry, len(m.StatusHistory))
	for i, h := range m.StatusHistory {
		history[i] = ecommerce.StatusEntry{
			Note:      h.Note,
			CreatedAt: h.CreatedAt,
		}
	}

	// Convert shipments
	var shipments []ecommerce.Shipment
	for _, s := range m.Shipments {
		for _, t := range s.Tracks {
			carrier := t.Title
			if carrier == "" {
				carrier = t.Carrier
			}
			shipments = append(shipments, ecommerce.Shipment{
				ID:             fmt.Sprintf("%d", s.ID),
				TrackingNumber: t.TrackNumber,
				Carrier:        carrier,
				CreatedAt:      parseTime(s.CreatedAt),
			})
		}
	}

	order := ecommerce.Order{
		ID:              fmt.Sprintf("%d", m.ID),
		IncrementID:     m.IncrementID,
		CustomerEmail:   m.CustomerEmail,
		CustomerName:    m.CustomerFirstname + " " + m.CustomerLastname,
		Status:          m.Status,
		State:           m.State,
		Items:           items,
		Subtotal:        m.Prices.Subtotal,
		GrandTotal:      m.Prices.GrandTotal,
		TotalPaid:       m.Prices.TotalPaid,
		TotalRefunded:   m.Prices.TotalRefunded,
		ShippingAmount:  m.Prices.ShippingAmount,
		Currency:        cur,
		PaymentMethod:   payMethod,
		ShippingMethod:  shipMethod,
		Shipments:       shipments,
		StatusHistory:   history,
		CreatedAt:       created,
	}
	if m.ShippingAddress != nil {
		order.ShippingAddress = convertAddress(m.ShippingAddress)
	}
	if m.BillingAddress != nil {
		order.BillingAddress = convertAddress(m.BillingAddress)
	}
	return order
}

func convertAddress(a *mahoAddress) *ecommerce.Address {
	return &ecommerce.Address{
		FirstName: a.FirstName,
		LastName:  a.LastName,
		Street:    strings.Join(a.Street, ", "),
		City:      a.City,
		Region:    a.Region,
		PostCode:  a.Postcode,
		Country:   a.CountryID,
		Telephone: a.Telephone,
	}
}

func parseTime(s string) time.Time {
	// Try ISO8601 first, then space-separated
	t, err := time.Parse("2006-01-02T15:04:05-07:00", s)
	if err != nil {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	return t
}
