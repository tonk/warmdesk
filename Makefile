BINARY   := coworker
DIST_DIR := dist
BACKEND  := backend
FRONTEND := frontend
VERSION  := $(shell git -C $(BACKEND) describe --tags --always 2>/dev/null || echo "dev")
ARCHIVE  := coworker-$(VERSION).tar.gz

.PHONY: all build build-frontend build-backend clean dev-backend dev-frontend run package

# Build everything into dist/
all: build

build: build-frontend build-backend
	@cp coworker.yaml.example $(DIST_DIR)/coworker.yaml.example
	@cp -r deploy $(DIST_DIR)/deploy
	@cp INSTALL.md $(DIST_DIR)/INSTALL.md
	@cp README.md $(DIST_DIR)/README.md
	@echo "Build complete. Output: $(DIST_DIR)/"

build-frontend:
	@echo "Building frontend..."
	cd $(FRONTEND) && npm install && npm run build
	mkdir -p $(DIST_DIR)/web
	cp -r $(FRONTEND)/dist/. $(DIST_DIR)/web/

build-backend:
	@echo "Building backend..."
	mkdir -p $(DIST_DIR)
	cd $(BACKEND) && go build -ldflags="-s -w" -o ../$(DIST_DIR)/$(BINARY) .

# Run in development mode (two terminals needed)
dev-backend:
	cd $(BACKEND) && go run .

dev-frontend:
	cd $(FRONTEND) && npm run dev

# Create distribution archive (run after build)
package: build
	@echo "Creating distribution archive $(ARCHIVE)..."
	@tar -czf $(ARCHIVE) -C $(DIST_DIR) .
	@echo "Distribution package: $(ARCHIVE)"

# Remove build artifacts
clean:
	rm -rf $(DIST_DIR) coworker-*.tar.gz

# Build and run production binary locally
run: build
	cd $(DIST_DIR) && WEB_DIR=./web ./$(BINARY)
