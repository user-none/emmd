import SwiftUI
import EblituiIOS

@main
struct emmdApp: App {
    @StateObject private var appState: AppState

    init() {
        EmulatorBridge.register(EmmdBridgeProvider.self)
        _appState = StateObject(wrappedValue: AppState())
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(appState)
                .preferredColorScheme(.dark)
        }
    }
}
