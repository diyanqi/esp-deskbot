.PHONY: run build test integration-test vet fmt vuln docker-build clean seed

run:
	set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/server

seed:
	set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/seed -hours 168 -interval 5m

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/server ./cmd/server

test:
	go test ./...

# Requires a disposable PostgreSQL database:
#   TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5432/data_collect_test?sslmode=disable make integration-test
integration-test:
	@test -n "$$TEST_DATABASE_URL" || (echo "TEST_DATABASE_URL is required" && exit 1)
	TEST_DATABASE_URL="$$TEST_DATABASE_URL" go test ./... -v

vet:
	go vet ./...

fmt:
	gofmt -w .

# Requires network access. If proxy.golang.org is unreachable:
#   GOPROXY=https://goproxy.cn,direct make vuln
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

docker-build:
	docker build -t data-collect-server:latest .

clean:
	rm -rf bin coverage.out
