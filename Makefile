SHELL := /bin/bash
ENV := set -a; [ -f .env ] && source .env; set +a;

.PHONY: deps db api api-memory ingest eval eval-real test web up image-api image-web

deps:            ## install Go + JS dependencies
	cd api && go mod tidy
	cd web && npm install

db:              ## start Postgres (schema auto-applied on first start)
	docker compose up -d db

api:             ## run the API against Postgres
	$(ENV) cd api && go run ./cmd/server

api-memory:      ## run the API with in-memory store + sample registry
	$(ENV) cd api && STORE=memory go run ./cmd/server

ingest:          ## load registry CSVs into Postgres
	$(ENV) cd api && go run ./cmd/ingest -agencies $$AGENCIES_CSV -flagged $$FLAGGED_CSV

eval:            ## run the eval set against a running API
	$(ENV) cd api && go run ./cmd/eval -api http://localhost:$${PORT:-8080} -cases ../eval/cases

eval-real:       ## run real-register cases (API must use data/real/agencies.csv)
	$(ENV) cd api && go run ./cmd/eval -api http://localhost:$${PORT:-8080} -cases ../eval/real

test:
	cd api && go vet ./... && go test ./...

web:
	cd web && npm run dev

up:              ## run API + web in Docker, as deployed (http://localhost:3000)
	docker compose --profile app up --build

image-api:       ## build the API image for linux/amd64 (cloud servers); IMAGE=<registry/name:tag>
	docker buildx build --platform linux/amd64 -f api/Dockerfile -t $${IMAGE:-kazi-halali-api:latest} --load .

image-web:       ## build the web image; API_URL=<public API URL> IMAGE=<registry/name:tag>
	docker buildx build --platform linux/amd64 --build-arg NEXT_PUBLIC_API_URL=$${API_URL:?set API_URL} -t $${IMAGE:-kazi-halali-web:latest} --load web