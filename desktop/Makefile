.PHONY: build

VERSION := $(shell cat ../VERSION)
GO_VERSION_LDFLAGS := -X github.com/ananthakumaran/paisa/internal/version.current=$(VERSION)

develop:
	wails dev -tags webkit2_40 -ldflags "$(GO_VERSION_LDFLAGS)"

build:
	wails build -tags webkit2_40 -ldflags "$(GO_VERSION_LDFLAGS)"

dmg:
	rm -rf Paisa.dmg
	wails build -ldflags "$(GO_VERSION_LDFLAGS)"
	create-dmg --volname "Paisa" --window-pos 200 120 --window-size 800 400 --icon-size 100  --icon "Paisa.app" 200 190 --app-drop-link 600 185 --hide-extension "Paisa.app" --volicon build/bin/Paisa.app/Contents/Resources/iconfile.icns  "Paisa.dmg" build/bin/Paisa.app
