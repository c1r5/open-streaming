GO      = go
GOCMD   = $(GO)
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST  = $(GOCMD) test
GORUN   = $(GOCMD) run

OUT = ../bin
BINARY  = server 
.PHONY: build test clean run deps tidy

deps:
	$(GOCMD) get github.com/go-gormigrate/gormigrate/v2
	$(GOCMD) get github.com/joho/godotenv
	$(GOCMD) get gorm.io/driver/sqlite
	$(GOCMD) get gorm.io/gorm
	$(GOCMD) mod tidy

tidy:
	$(GOCMD) mod tidy

build:
	$(GOBUILD) -tags go_sqlite3 -o $(OUT)/$(BINARY) ./src/main.go

test:
	$(GOTEST) ./...

clean:
	$(GOCLEAN)
	rm -f $(OUT)/$(BINARY)

run: build
	$(OUT)/$(BINARY)

deploy:
	@docker compose down
	@docker compose up --build -d