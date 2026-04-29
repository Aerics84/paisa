.PHONY: docs
.PHONY: fixture/main.transactions.json
.PHONY: preflight-critical
.PHONY: prepush

VERSION := $(shell cat VERSION)
GO_VERSION_LDFLAGS := -ldflags "-X github.com/ananthakumaran/paisa/internal/version.current=$(VERSION)"

develop:
	./node_modules/.bin/concurrently --names "GO,JS" -c "auto" "make serve" "npm run dev"

serve:
	./node_modules/.bin/nodemon --signal SIGTERM --delay 2000ms --watch '.' --ext go,json --exec 'go run . serve || exit 1'

debug:
	./node_modules/.bin/concurrently --names "GO,JS" -c "auto" "make serve-now" "npm run dev"

serve-now:
	./node_modules/.bin/nodemon --signal SIGTERM --delay 2000ms --watch '.' --ext go,json --exec 'TZ=UTC go run . serve --now 2022-02-07 || exit 1'


watch:
	npm run "build:watch"
docs:
	mkdocs serve -a 0.0.0.0:8000

sample:
	go build $(GO_VERSION_LDFLAGS) && ./paisa init && ./paisa update

publish:
	nix develop --command bash -c 'mkdocs build'

parser:
	npm run parser-build-debug

lint:
	npm run check:format:changed
	npm run check
	bash ./scripts/gofmt-changed.sh

preflight-critical:
	GO_VERSION_LDFLAGS='$(GO_VERSION_LDFLAGS)' bash ./scripts/preflight-critical.sh

prepush: preflight-critical

regen:
	go build $(GO_VERSION_LDFLAGS)
	unset PAISA_CONFIG && REGENERATE=true TZ=UTC bun test tests

jstest:
	bun test --preload ./src/happydom.ts src
	go build $(GO_VERSION_LDFLAGS)
	unset PAISA_CONFIG && TZ=UTC bun test tests

jsbuild:
	npm run build

test: jsbuild jstest
	go test ./...

windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CXX=x86_64-w64-mingw32-g++ CC=x86_64-w64-mingw32-gcc go build $(GO_VERSION_LDFLAGS)


deploy:
	fly scale count 2 --region lax --yes
	docker build -t paisa . --file Dockerfile.demo
	fly deploy -i paisa:latest --local-only
	fly scale count 1 --region lax --yes

install:
	npm run build
	go build $(GO_VERSION_LDFLAGS)
	go install $(GO_VERSION_LDFLAGS)

fixture/main.transactions.json:
	cd /tmp && paisa init
	cp fixture/main.ledger /tmp/main.ledger
	cd /tmp && paisa update --journal && paisa serve -p 6500 &
	sleep 1
	curl http://localhost:6500/api/transaction | jq .transactions > fixture/main.transactions.json
	pkill -f 'paisa serve -p 6500'

generate-fonts:
	npm --prefix tools/fontgen install
	npm --prefix tools/fontgen run generate-fonts

node2nix:
	npm install --lockfile-version 2
	node2nix --development -18 --input package.json \
	--lock package-lock.json \
	--node-env ./flake/node-env.nix \
	--composition ./flake/default.nix \
	--output ./flake/node-package.nix
