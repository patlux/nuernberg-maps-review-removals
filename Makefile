.PHONY: setup test check validate scrape backfill charts dashboard site deploy-pages all

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

deploy-pages: site
	@tmp=$$(mktemp -d); \
	git clone --quiet --branch gh-pages --single-branch $$(git remote get-url origin) $$tmp; \
	git -C $$tmp rm -r --ignore-unmatch . >/dev/null; \
	cp -R public/. $$tmp/; \
	git -C $$tmp add -A; \
	if git -C $$tmp diff --cached --quiet; then \
		echo "gh-pages ist bereits aktuell"; \
	else \
		git -C $$tmp commit -m "Deploy GitHub Pages site"; \
		git -C $$tmp push origin gh-pages; \
	fi; \
	rm -rf $$tmp

all:
	go run ./cmd/scrape --postcodes all
	go run ./cmd/charts --png
	go run ./cmd/dashboard
