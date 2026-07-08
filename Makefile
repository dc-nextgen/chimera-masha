# Masha agent — Go tek-binary DB connector (yerel yuz React+shadcn, binary'ye EMBED).
BIN := masha-agent

.PHONY: build web go test dev run clean

build: web go   ## UI'yi derle + binary'ye embed et (uretim)

web:            ## React+shadcn yuzu derle → web/dist
	cd web && npm install && npm run build

go:             ## Go binary (web/dist embed)
	go build -o $(BIN) .

test:           ## Go testleri
	go test ./...

dev:            ## Vite dev sunucu (5173; /try·/schema·/healthz → 8787 proxy). Ayrica `make run` gerekir.
	cd web && npm run dev

run:            ## agent'i yerel test icin calistir (.env.test + ornek manifest)
	set -a; . ./.env.test; set +a; MASHA_MANIFEST=./manifests/simetrit.json ./$(BIN) serve

clean:
	rm -f $(BIN)
	rm -rf web/node_modules
	find web/dist -mindepth 1 ! -name .gitkeep -delete 2>/dev/null || true
