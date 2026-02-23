FRAMEWORK_DIR = Frameworks
XCFRAMEWORK = $(FRAMEWORK_DIR)/Emulator.xcframework

.PHONY: framework build simulator clean

framework:
	mkdir -p $(FRAMEWORK_DIR)
	gomobile bind -target ios \
		-o $(XCFRAMEWORK) ../cmd/ios

build: framework
	xcodebuild -project emmd.xcodeproj -scheme emmd \
		-destination 'generic/platform=iOS' build

simulator: framework
	xcodebuild -project emmd.xcodeproj -scheme emmd \
		-destination 'platform=iOS Simulator,name=iPhone 16' build

clean:
	rm -rf $(FRAMEWORK_DIR)
