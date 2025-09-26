package main

import (
	"database/sql"
	"encoding/json"
	"time"
)

// User represents a user in the system
type User struct {
	ID        int64     `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Email     string    `json:"email" db:"email"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	Active    bool      `json:"active" db:"active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// FullName returns the user's full name
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}

// IsActive checks if the user account is active
func (u *User) IsActive() bool {
	return u.Active
}

// ToJSON converts the user to JSON representation
func (u *User) ToJSON() ([]byte, error) {
	return json.Marshal(u)
}

// Product represents a product in the inventory
type Product struct {
	ID          int64           `json:"id" db:"id"`
	SKU         string          `json:"sku" db:"sku"`
	Name        string          `json:"name" db:"name"`
	Description sql.NullString  `json:"description" db:"description"`
	Price       float64         `json:"price" db:"price"`
	CategoryID  int64           `json:"category_id" db:"category_id"`
	InStock     bool            `json:"in_stock" db:"in_stock"`
	Quantity    int             `json:"quantity" db:"quantity"`
	Tags        []string        `json:"tags" db:"tags"`
	Metadata    json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// IsAvailable checks if the product is available for purchase
func (p *Product) IsAvailable() bool {
	return p.InStock && p.Quantity > 0
}

// FormattedPrice returns the price formatted as currency
func (p *Product) FormattedPrice() string {
	return fmt.Sprintf("$%.2f", p.Price)
}

// HasTag checks if the product has a specific tag
func (p *Product) HasTag(tag string) bool {
	for _, t := range p.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// Category represents a product category
type Category struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	ParentID    *int64    `json:"parent_id" db:"parent_id"`
	Active      bool      `json:"active" db:"active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Order represents a customer order
type Order struct {
	ID         int64       `json:"id" db:"id"`
	UserID     int64       `json:"user_id" db:"user_id"`
	Status     OrderStatus `json:"status" db:"status"`
	TotalPrice float64     `json:"total_price" db:"total_price"`
	Items      []OrderItem `json:"items"`
	ShippingAddress Address `json:"shipping_address"`
	BillingAddress  Address `json:"billing_address"`
	CreatedAt  time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at" db:"updated_at"`
}

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// OrderItem represents an item within an order
type OrderItem struct {
	ID        int64   `json:"id" db:"id"`
	OrderID   int64   `json:"order_id" db:"order_id"`
	ProductID int64   `json:"product_id" db:"product_id"`
	Quantity  int     `json:"quantity" db:"quantity"`
	UnitPrice float64 `json:"unit_price" db:"unit_price"`
	TotalPrice float64 `json:"total_price" db:"total_price"`
}

// CalculateTotal calculates the total price for the order item
func (oi *OrderItem) CalculateTotal() {
	oi.TotalPrice = float64(oi.Quantity) * oi.UnitPrice
}

// Address represents a physical address
type Address struct {
	ID         int64  `json:"id" db:"id"`
	Street     string `json:"street" db:"street"`
	City       string `json:"city" db:"city"`
	State      string `json:"state" db:"state"`
	PostalCode string `json:"postal_code" db:"postal_code"`
	Country    string `json:"country" db:"country"`
}

// FormatAddress returns a formatted address string
func (a *Address) FormatAddress() string {
	return fmt.Sprintf("%s, %s, %s %s, %s", a.Street, a.City, a.State, a.PostalCode, a.Country)
}

// Repository interfaces for data access

// UserRepository defines methods for user data operations
type UserRepository interface {
	GetByID(id int64) (*User, error)
	GetByUsername(username string) (*User, error)
	Create(user *User) error
	Update(user *User) error
	Delete(id int64) error
	List(limit, offset int) ([]*User, error)
}

// ProductRepository defines methods for product data operations
type ProductRepository interface {
	GetByID(id int64) (*Product, error)
	GetBySKU(sku string) (*Product, error)
	GetByCategory(categoryID int64, limit, offset int) ([]*Product, error)
	Create(product *Product) error
	Update(product *Product) error
	Delete(id int64) error
	Search(query string, limit, offset int) ([]*Product, error)
}

// OrderRepository defines methods for order data operations
type OrderRepository interface {
	GetByID(id int64) (*Order, error)
	GetByUserID(userID int64, limit, offset int) ([]*Order, error)
	Create(order *Order) error
	Update(order *Order) error
	UpdateStatus(id int64, status OrderStatus) error
	Delete(id int64) error
}