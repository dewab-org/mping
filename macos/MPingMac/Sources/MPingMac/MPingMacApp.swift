import SwiftUI
import AppKit

extension Notification.Name {
    static let commandEditHosts = Notification.Name("mping.command.editHosts")
    static let commandResetHosts = Notification.Name("mping.command.resetHosts")
    static let commandDeleteHosts = Notification.Name("mping.command.deleteHosts")
    static let commandEditPort = Notification.Name("mping.command.editPort")
}

@main
struct MPingMacApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var store = HostStore()
    @AppStorage("showStatusBar") private var showStatusBar: Bool = true

    var body: some Scene {
        WindowGroup {
            ContentView(store: store, showStatusBar: $showStatusBar)
                .onAppear {
                    appDelegate.bringToFront()
                }
        }
        .windowResizability(.contentSize)

        Settings {
            SettingsSceneView(store: store)
        }
        .windowResizability(.automatic)

        .commands {
            CommandGroup(replacing: .appInfo) {
                Button("About MPingMac") {
                    AboutPresenter.show()
                }
            }
            CommandGroup(after: .newItem) {
                Button("Open Hosts…") {
                    OpenFileHelper.openHostsFile(store: store)
                }
                .keyboardShortcut("o", modifiers: [.command])

                Button("Clear Hosts") {
                    store.clearHosts()
                }
                .keyboardShortcut(.delete, modifiers: [.command, .shift])
            }

            CommandGroup(after: .toolbar) {
                Toggle("Show Status Bar", isOn: $showStatusBar)
            }

            CommandGroup(after: .pasteboard) {
                Button("Reset Selected Host") {
                    NotificationCenter.default.post(name: .commandResetHosts, object: nil)
                }
                .keyboardShortcut("r", modifiers: [.command])
                .disabled(store.selection.isEmpty)

                Button("Delete Selected Host") {
                    NotificationCenter.default.post(name: .commandDeleteHosts, object: nil)
                }
                .keyboardShortcut(.delete, modifiers: [.command])
                .disabled(store.selection.isEmpty)

                Button("Edit Hosts…") {
                    NotificationCenter.default.post(name: .commandEditHosts, object: nil)
                }
                .keyboardShortcut("=", modifiers: [.command, .shift])

                Button("Change TCP Port…") {
                    NotificationCenter.default.post(name: .commandEditPort, object: nil)
                }
                .keyboardShortcut("p", modifiers: [.command])
            }
        }
    }
}

private struct SettingsSceneView: View {
    @Environment(\.dismiss) private var dismiss
    @ObservedObject var store: HostStore

    var body: some View {
        SettingsView(store: store) {
            dismiss()
        }
        .padding()
        .frame(minWidth: 520)
    }
}

enum OpenFileHelper {
    static func openHostsFile(store: HostStore) {
        let panel = NSOpenPanel()
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        panel.canChooseFiles = true
        panel.allowedContentTypes = [.plainText]

        panel.begin { response in
            guard response == .OK, let url = panel.url else { return }
            Task { @MainActor in
                if let contents = try? String(contentsOf: url) {
                    _ = store.addHosts(from: contents)
                }
            }
        }
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {

    func applicationDidFinishLaunching(_ notification: Notification) {
        applyAppIconIfAvailable()
        bringToFront()
    }

    func applicationDidBecomeActive(_ notification: Notification) {
        applyAppIconIfAvailable()
        bringToFront()
    }

    @MainActor
    func bringToFront() {
        NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)
        NSApp.windows.forEach { window in
            guard window.isVisible else { return }
            window.makeKeyAndOrderFront(nil)
        }
    }

    private func applyAppIconIfAvailable() {
        guard let image = loadIconImage() else { return }
        NSApp.applicationIconImage = image
    }

    private func loadIconImage() -> NSImage? {
        let bundles: [Bundle] = [Bundle.module, Bundle.main]
        for bundle in bundles {
            if let url = bundle.url(forResource: "AppIcon", withExtension: "icns"),
               let image = NSImage(contentsOf: url) {
                return image
            }
            if let image = bundle.image(forResource: NSImage.Name("AppIcon")) {
                return image
            }
        }
        return nil
    }

}

enum AboutPresenter {
    static func show() {
        let icon = NSApp.applicationIconImage ?? NSImage(named: "AppIcon")
        let credits = creditsString()
        let options: [NSApplication.AboutPanelOptionKey: Any] = [
            .applicationName: "MPingMac",
            .applicationVersion: "v0.1",
            .applicationIcon: icon as Any,
            .credits: credits
        ]
        NSApp.orderFrontStandardAboutPanel(options: options)
        NSApp.activate(ignoringOtherApps: true)
    }

    private static func creditsString() -> NSAttributedString {
        let text = "Daniel Whicker\n© 2025"
        return NSAttributedString(string: text, attributes: [
            .font: NSFont.systemFont(ofSize: 12),
            .foregroundColor: NSColor.labelColor
        ])
    }
}
