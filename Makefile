.PHONY: setup test check validate scrape backfill charts dashboard site all

setup:
	go mod download

test:
	go test ./...

check:
	go test ./...
	go run ./cmd/validate

validate:
	go run ./cmd/validate $(ARGS)

scrape:
	go run ./cmd/scrape $(ARGS)

backfill:
	go run ./cmd/backfill $(ARGS)

charts:
	go run ./cmd/charts $(ARGS)

dashboard:
	go run ./cmd/dashboard $(ARGS)

site: charts dashboard
	rm -rf public
	mkdir -p public/charts public/data
	touch public/.nojekyll
	cp output/charts/nuernberg_dashboard.html public/index.html
	cp output/charts/* public/charts/
	cp output/metadata.json output/places.csv public/data/

all:
	go run ./cmd/scrape --postcodes all
	go run ./cmd/charts --png
	go run ./cmd/dashboard
