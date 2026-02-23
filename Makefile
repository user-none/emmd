.PHONY: all clean libretro standalone macos icons iconset

# Output directories
BUILD_DIR := build
ICONSET_DIR := $(BUILD_DIR)/icon.iconset
APP_NAME := emmd
APP_BUNDLE := $(BUILD_DIR)/$(APP_NAME).app

# Source files
ICON_MASTER := assets/icon.png
ICON_ICNS := $(BUILD_DIR)/icon.icns
IOS_ICON := ios/emmd/Resources/Assets.xcassets/AppIcon.appiconset/icon.png

# Build all targets
all: libretro standalone

# Build the standalone binary
standalone:
	go build -o $(BUILD_DIR)/emmd ./cmd/standalone/

# Build macOS .app bundle
macos: standalone icons
	@echo "Creating $(APP_NAME).app bundle..."
	@mkdir -p "$(APP_BUNDLE)/Contents/MacOS"
	@mkdir -p "$(APP_BUNDLE)/Contents/Resources"
	@cp $(BUILD_DIR)/emmd "$(APP_BUNDLE)/Contents/MacOS/"
	@cp $(ICON_ICNS) "$(APP_BUNDLE)/Contents/Resources/icon.icns"
	@cp assets/macos_info.plist "$(APP_BUNDLE)/Contents/Info.plist"
	@echo "APPL????" > "$(APP_BUNDLE)/Contents/PkgInfo"
	@echo "Signing app bundle..."
	@codesign --force --sign - --deep "$(APP_BUNDLE)"
	@echo "Created $(APP_BUNDLE)"

# Build libretro core
libretro:
	go build -buildmode=c-shared -o $(BUILD_DIR)/emmd_libretro.dylib ./cmd/libretro/

# Generate icons from master PNG
icons: $(ICON_ICNS) $(IOS_ICON)

# iOS icon (just copy the 1024x1024 master)
$(IOS_ICON): $(ICON_MASTER)
	@echo "Copying icon to iOS..."
	@cp $(ICON_MASTER) $(IOS_ICON)

$(ICON_ICNS): $(ICON_MASTER) | $(BUILD_DIR)
	@echo "Generating macOS icon..."
	@mkdir -p $(ICONSET_DIR)
	@sips -z 16 16 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_16x16.png
	@sips -z 32 32 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_16x16@2x.png
	@sips -z 32 32 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_32x32.png
	@sips -z 64 64 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_32x32@2x.png
	@sips -z 128 128 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_128x128.png
	@sips -z 256 256 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_128x128@2x.png
	@sips -z 256 256 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_256x256.png
	@sips -z 512 512 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_256x256@2x.png
	@sips -z 512 512 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_512x512.png
	@sips -z 1024 1024 $(ICON_MASTER) --out $(ICONSET_DIR)/icon_512x512@2x.png
	@iconutil -c icns $(ICONSET_DIR) -o $(ICON_ICNS)
	@rm -rf $(ICONSET_DIR)
	@echo "Created $(ICON_ICNS)"

$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

clean:
	rm -rf $(BUILD_DIR)
