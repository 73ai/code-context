package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DatabaseName string
	SSLMode      string
	MaxIdleConns int
	MaxOpenConns int
	MaxLifetime  time.Duration
}

// Database wraps sql.DB with additional functionality
type Database struct {
	db     *sql.DB
	config *DatabaseConfig
}

// NewDatabase creates a new database connection
func NewDatabase(config *DatabaseConfig) (*Database, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DatabaseName, config.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetConnMaxLifetime(config.MaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{db: db, config: config}, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// Ping checks if the database connection is alive
func (d *Database) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// UserDatabaseRepository implements UserRepository using SQL database
type UserDatabaseRepository struct {
	db *Database
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *Database) *UserDatabaseRepository {
	return &UserDatabaseRepository{db: db}
}

// GetByID retrieves a user by ID
func (r *UserDatabaseRepository) GetByID(id int64) (*User, error) {
	query := `
		SELECT id, username, email, first_name, last_name, active, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	user := &User{}
	err := r.db.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.FirstName,
		&user.LastName, &user.Active, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetByUsername retrieves a user by username
func (r *UserDatabaseRepository) GetByUsername(username string) (*User, error) {
	query := `
		SELECT id, username, email, first_name, last_name, active, created_at, updated_at
		FROM users
		WHERE username = $1
	`

	user := &User{}
	err := r.db.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.FirstName,
		&user.LastName, &user.Active, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// Create creates a new user
func (r *UserDatabaseRepository) Create(user *User) error {
	query := `
		INSERT INTO users (username, email, first_name, last_name, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	err := r.db.db.QueryRow(
		query, user.Username, user.Email, user.FirstName, user.LastName,
		user.Active, user.CreatedAt, user.UpdatedAt,
	).Scan(&user.ID)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// Update updates an existing user
func (r *UserDatabaseRepository) Update(user *User) error {
	query := `
		UPDATE users
		SET username = $1, email = $2, first_name = $3, last_name = $4, active = $5, updated_at = $6
		WHERE id = $7
	`

	user.UpdatedAt = time.Now()

	result, err := r.db.db.Exec(
		query, user.Username, user.Email, user.FirstName, user.LastName,
		user.Active, user.UpdatedAt, user.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// Delete deletes a user by ID
func (r *UserDatabaseRepository) Delete(id int64) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// List retrieves users with pagination
func (r *UserDatabaseRepository) List(limit, offset int) ([]*User, error) {
	query := `
		SELECT id, username, email, first_name, last_name, active, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.FirstName,
			&user.LastName, &user.Active, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// ProductDatabaseRepository implements ProductRepository using SQL database
type ProductDatabaseRepository struct {
	db *Database
}

// NewProductRepository creates a new product repository
func NewProductRepository(db *Database) *ProductDatabaseRepository {
	return &ProductDatabaseRepository{db: db}
}

// GetByID retrieves a product by ID
func (r *ProductDatabaseRepository) GetByID(id int64) (*Product, error) {
	query := `
		SELECT id, sku, name, description, price, category_id, in_stock, quantity,
		       tags, metadata, created_at, updated_at
		FROM products
		WHERE id = $1
	`

	product := &Product{}
	err := r.db.db.QueryRow(query, id).Scan(
		&product.ID, &product.SKU, &product.Name, &product.Description,
		&product.Price, &product.CategoryID, &product.InStock, &product.Quantity,
		&product.Tags, &product.Metadata, &product.CreatedAt, &product.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("product not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return product, nil
}

// Search performs a full-text search on products
func (r *ProductDatabaseRepository) Search(query string, limit, offset int) ([]*Product, error) {
	searchQuery := `
		SELECT id, sku, name, description, price, category_id, in_stock, quantity,
		       tags, metadata, created_at, updated_at
		FROM products
		WHERE to_tsvector('english', name || ' ' || COALESCE(description::text, ''))
		      @@ plainto_tsquery('english', $1)
		ORDER BY ts_rank(to_tsvector('english', name || ' ' || COALESCE(description::text, '')),
		                 plainto_tsquery('english', $1)) DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.db.Query(searchQuery, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search products: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		product := &Product{}
		err := rows.Scan(
			&product.ID, &product.SKU, &product.Name, &product.Description,
			&product.Price, &product.CategoryID, &product.InStock, &product.Quantity,
			&product.Tags, &product.Metadata, &product.CreatedAt, &product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, product)
	}

	return products, nil
}

// Transaction helper functions

// WithTransaction executes a function within a database transaction
func (d *Database) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// Migration utilities

// Migration represents a database migration
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// MigrationManager handles database migrations
type MigrationManager struct {
	db *Database
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *Database) *MigrationManager {
	return &MigrationManager{db: db}
}

// RunMigrations executes pending migrations
func (m *MigrationManager) RunMigrations(migrations []Migration) error {
	if err := m.createMigrationTable(); err != nil {
		return err
	}

	for _, migration := range migrations {
		if applied, err := m.isMigrationApplied(migration.Version); err != nil {
			return err
		} else if applied {
			continue
		}

		log.Printf("Running migration %d: %s", migration.Version, migration.Name)

		if err := m.executeMigration(migration); err != nil {
			return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
		}

		if err := m.markMigrationApplied(migration); err != nil {
			return fmt.Errorf("failed to mark migration %d as applied: %w", migration.Version, err)
		}
	}

	return nil
}

// createMigrationTable creates the migration tracking table
func (m *MigrationManager) createMigrationTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := m.db.db.Exec(query)
	return err
}

// isMigrationApplied checks if a migration has been applied
func (m *MigrationManager) isMigrationApplied(version int) (bool, error) {
	var count int
	err := m.db.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&count)
	return count > 0, err
}

// executeMigration executes a migration SQL
func (m *MigrationManager) executeMigration(migration Migration) error {
	statements := strings.Split(migration.SQL, ";")
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := m.db.db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

// markMigrationApplied marks a migration as applied
func (m *MigrationManager) markMigrationApplied(migration Migration) error {
	query := `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`
	_, err := m.db.db.Exec(query, migration.Version, migration.Name)
	return err
}