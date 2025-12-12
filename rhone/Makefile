.PHONY: all generate sqlc templ css build test dev clean migrate-up migrate-down

# Default target
all: generate build

# Generate all code (sqlc, templ, css)
generate: sqlc templ css

# Generate database code from queries.sql
sqlc:
	sqlc generate

# Generate Go code from .templ files
templ:
	templ generate

# Build Tailwind CSS
css:
	npm run build:css

# Build binary
build: generate
	go build -o bin/rhone ./cmd/rhone

# Run tests
test: generate
	go test -v ./...

# Run locally with hot reload
dev:
	@echo "Starting development server..."
	@go run ./cmd/rhone

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f internal/database/queries/db.go
	rm -f internal/database/queries/models.go
	rm -f internal/database/queries/queries.sql.go
	rm -f internal/templates/**/*_templ.go

# Run database migrations up
migrate-up:
	migrate -path internal/database/migrations -database "$(DATABASE_URL)" up

# Run database migrations down
migrate-down:
	migrate -path internal/database/migrations -database "$(DATABASE_URL)" down

# Install development tools
install-tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/a-h/templ/cmd/templ@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	npm install
