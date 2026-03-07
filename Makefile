TAG_NAME?=$(shell git describe --tags --dirty --abbrev=0 2>/dev/null || echo $(shell git rev-parse --short HEAD)-dirty)
APP_NAME="Debrief"
APP_NAME_LOWERCASE="debrief"
LINUX_ARCH?=amd64

ifeq ($(LINUX_ARCH),arm64)
  LINUX_CC?=aarch64-linux-gnu-gcc
else
  LINUX_CC?=gcc
endif

.PHONY: build_macos_app
build_macos_app:
	@echo "Building macOS..."
	gogio -ldflags="-X version.AppVersion=$(TAG_NAME)" -appid=rest.${APP_NAME_LOWERCASE}.app -icon=./assets/appicon.png -target=macos -arch=amd64 -o ./dist/amd64/${APP_NAME}.app .
	gogio -ldflags="-X version.AppVersion=$(TAG_NAME)" -appid=rest.${APP_NAME_LOWERCASE}.app -icon=./assets/appicon.png -target=macos -arch=arm64 -o ./dist/arm64/${APP_NAME}.app .
	cp ./assets/appicon.icns ./dist/amd64/${APP_NAME}.app/Contents/Resources/appicon.icns
	cp ./assets/appicon.icns ./dist/arm64/${APP_NAME}.app/Contents/Resources/appicon.icns
	codesign --force --deep --sign - ./dist/amd64/${APP_NAME}.app
	codesign --force --deep --sign - ./dist/arm64/${APP_NAME}.app

.PHONY: build_macos_dmg
build_macos_dmg:
	@echo "Building macOS DMG..."
	rm -rf ./dist/${APP_NAME_LOWERCASE}-macos-$(TAG_NAME)-amd64.dmg
	rm -rf ./dist/${APP_NAME_LOWERCASE}-macos-$(TAG_NAME)-arm64.dmg
	create-dmg \
	  --volname "${APP_NAME} Installer" \
	  --volicon "./assets/appicon.icns" \
	  --background "./assets/${APP_NAME_LOWERCASE}-installer-bk.png" \
	  --window-pos 300 300 \
	  --window-size 500 350 \
	  --icon-size 100 \
	  --icon "${APP_NAME}.app" 125 150 \
	  --hide-extension "${APP_NAME}.app" \
	  --no-internet-enable \
	  --app-drop-link 375 150 \
	  "./dist/${APP_NAME_LOWERCASE}-macos-$(TAG_NAME)-arm64.dmg" \
	  "./dist/arm64/${APP_NAME}.app"

	create-dmg \
	  --volname "${APP_NAME} Installer" \
	  --volicon "./assets/appicon.icns" \
	  --background "./assets/${APP_NAME_LOWERCASE}-installer-bk.png" \
	  --window-pos 300 300 \
	  --window-size 500 350 \
	  --icon-size 100 \
	  --icon "${APP_NAME}.app" 125 150 \
	  --hide-extension "${APP_NAME}.app" \
	  --no-internet-enable \
	  --app-drop-link 375 150 \
	  "./dist/${APP_NAME_LOWERCASE}-macos-$(TAG_NAME)-amd64.dmg" \
	  "./dist/amd64/${APP_NAME}.app"

.PHONY: build_macos_signed
build_macos_signed:
	@echo "Building and signing macOS app..."
	@if [ -z "$(APPLE_TEAM_ID)" ]; then \
		echo "ERROR: APPLE_TEAM_ID environment variable is not set"; \
		exit 1; \
	fi
	@if [ -z "$(IDENTITY)" ]; then \
		echo "ERROR: IDENTITY environment variable is not set (e.g., 'Developer ID Application: Your Name (TEAMID)')"; \
		exit 1; \
	fi

	@echo "Building amd64..."
	gogio -ldflags="-X version.AppVersion=$(TAG_NAME)" -appid=rest.${APP_NAME_LOWERCASE}.app -icon=./assets/appicon.png -target=macos -arch=amd64 -o ./dist/amd64/${APP_NAME}.app .
	@echo "Building arm64..."
	gogio -ldflags="-X version.AppVersion=$(TAG_NAME)" -appid=rest.${APP_NAME_LOWERCASE}.app -icon=./assets/appicon.png -target=macos -arch=arm64 -o ./dist/arm64/${APP_NAME}.app .

	@echo "Replacing icons..."
	cp ./assets/appicon.icns ./dist/amd64/${APP_NAME}.app/Contents/Resources/appicon.icns
	cp ./assets/appicon.icns ./dist/arm64/${APP_NAME}.app/Contents/Resources/appicon.icns

	@echo "Signing amd64..."
	xattr -cr ./dist/amd64/${APP_NAME}.app && codesign --force --options runtime --deep -vvv --sign "$(IDENTITY)" ./dist/amd64/${APP_NAME}.app
	@echo "Signing arm64..."
	xattr -cr ./dist/arm64/${APP_NAME}.app && codesign --force --options runtime --deep --sign "$(IDENTITY)" ./dist/arm64/${APP_NAME}.app

	@echo "Verifying signatures..."
	codesign --verify --deep --strict ./dist/amd64/${APP_NAME}.app
	codesign --verify --deep --strict ./dist/arm64/${APP_NAME}.app

	@echo "Apps built and signed. To notarize, run:"
	@echo "  ditto -c -k --keepParent ./dist/amd64/${APP_NAME}.app ./dist/${APP_NAME}-amd64.zip"
	@echo "  xcrun notarytool submit ./dist/${APP_NAME}-amd64.zip --apple-id \"\$$APPLE_ID\" --password \"\$$APPLE_APP_SPECIFIC_PASSWORD\" --team-id \"\$$APPLE_TEAM_ID\" --wait"
	@echo "  xcrun stapler staple ./dist/amd64/${APP_NAME}.app"

	@echo "Then create DMG with 'make build_macos_dmg'"

.PHONY: notarize_macos
notarize_macos:
	@echo "Notarizing macOS apps..."
	@if [ -z "$(APPLE_ID)" ] || [ -z "$(APPLE_TEAM_ID)" ] || [ -z "$(APPLE_APP_SPECIFIC_PASSWORD)" ]; then \
		echo "ERROR: One or more required environment variables are not set:"; \
		echo "  - APPLE_ID"; \
		echo "  - APPLE_TEAM_ID"; \
		echo "  - APPLE_APP_SPECIFIC_PASSWORD"; \
		exit 1; \
	fi

	@echo "Creating zip archives..."
	ditto -c -k --keepParent ./dist/amd64/${APP_NAME}.app ./dist/${APP_NAME}-amd64.zip
	ditto -c -k --keepParent ./dist/arm64/${APP_NAME}.app ./dist/${APP_NAME}-arm64.zip

	@echo "Submitting amd64 for notarization..."
	@xcrun notarytool submit ./dist/${APP_NAME}-amd64.zip --apple-id "$(APPLE_ID)" --password "$(APPLE_APP_SPECIFIC_PASSWORD)" --team-id "$(APPLE_TEAM_ID)" --wait --verbose
	@echo "Submitting arm64 for notarization..."
	@xcrun notarytool submit ./dist/${APP_NAME}-arm64.zip --apple-id "$(APPLE_ID)" --password "$(APPLE_APP_SPECIFIC_PASSWORD)" --team-id "$(APPLE_TEAM_ID)" --wait --verbose

	@echo "Notarization complete. Now run 'make build_macos_dmg' to create DMG files."

.PHONY: build_appstore
build_appstore:
	@echo "Building for App Store..."
	@if [ -z "$(APPLE_TEAM_ID)" ]; then \
		echo "ERROR: APPLE_TEAM_ID environment variable is not set"; \
		exit 1; \
	fi
	@if [ -z "$(TAG_NAME)" ]; then \
		echo "ERROR: TAG_NAME environment variable is not set"; \
		exit 1; \
	fi

	./assets/build_appstore.sh $(TAG_NAME)


.PHONY: upload_appstore
upload_appstore:
	@echo "Uploading to App Store Connect..."
	@if [ -z "$(APPLE_ID)" ] || [ -z "$(APPLE_APP_SPECIFIC_PASSWORD)" ] || [ -z "$(TAG_NAME)" ]; then \
		echo "ERROR: One or more required environment variables are not set:"; \
		echo "  - APPLE_ID"; \
		echo "  - APPLE_APP_SPECIFIC_PASSWORD"; \
		echo "  - TAG_NAME"; \
		exit 1; \
	fi

	xcrun altool --upload-app -f ./dist/${APP_NAME}-$(TAG_NAME).pkg -t macos -u "$(APPLE_ID)" -p "$(APPLE_APP_SPECIFIC_PASSWORD)" --verbose


.PHONY: build_windows
build_windows:
	@echo "Building Windows..."
	rm -f *.syso
	cp assets/appicon.png .
	gogio -ldflags="-X version.AppVersion=${TAG_NAME}" -target=windows -arch=amd64 -o "dist/amd64/${APP_NAME}.exe" .
	gogio -ldflags="-X version.AppVersion=${TAG_NAME}" -target=windows -arch=arm64 -o "dist/arm64/${APP_NAME}.exe" .
	rm -f *.syso

.PHONY: build_linux_binary
build_linux_binary:
	@echo "Building Linux $(LINUX_ARCH) binary..."
	CGO_ENABLED=1 CC=$(LINUX_CC) GOOS=linux GOARCH=$(LINUX_ARCH) go build -ldflags="-X version.AppVersion=$(TAG_NAME)" -o ./dist/linux/${APP_NAME_LOWERCASE} .

.PHONY: build_linux
build_linux: build_linux_binary
	@echo "Packaging Linux $(LINUX_ARCH) tarball..."
	cp ./assets/install_linux.sh ./dist/linux
	cp ./assets/appicon.png ./dist/linux
	cp ./LICENSE ./dist/linux
	cp -r ./assets/desktop-assets ./dist/linux
	tar -cJf ./dist/${APP_NAME_LOWERCASE}-$(TAG_NAME)-$(LINUX_ARCH).tar.xz --directory=./dist/linux ${APP_NAME_LOWERCASE} desktop-assets install_linux.sh appicon.png ./LICENSE
	rm -rf ./dist/linux

.PHONY: build_deb
build_deb: build_linux_binary
	@echo "Building deb $(LINUX_ARCH) package..."
	ARCH=$(LINUX_ARCH) VERSION=$(TAG_NAME:v%=%) nfpm package --packager deb --target ./dist/${APP_NAME_LOWERCASE}-$(TAG_NAME)-$(LINUX_ARCH).deb
	rm -rf ./dist/linux

.PHONY: build_rpm
build_rpm: build_linux_binary
	@echo "Building rpm $(LINUX_ARCH) package..."
	ARCH=$(LINUX_ARCH) VERSION=$(TAG_NAME:v%=%) nfpm package --packager rpm --target ./dist/${APP_NAME_LOWERCASE}-$(TAG_NAME)-$(LINUX_ARCH).rpm
	rm -rf ./dist/linux

.PHONY: build_archlinux
build_archlinux: build_linux_binary
	@echo "Building Arch Linux $(LINUX_ARCH) package..."
	ARCH=$(LINUX_ARCH) VERSION=$(TAG_NAME:v%=%) nfpm package --packager archlinux --target ./dist/${APP_NAME_LOWERCASE}-$(TAG_NAME)-$(LINUX_ARCH).pkg.tar.zst
	rm -rf ./dist/linux

.PHONY: run
run:
	@echo "Running..."
	go run .

.PHONY: install_deps
install_deps:
	go install gioui.org/cmd/gogio@latest
