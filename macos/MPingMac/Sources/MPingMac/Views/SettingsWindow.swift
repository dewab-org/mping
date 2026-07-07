import AppKit
import SwiftUI

final class SettingsWindowController: NSWindowController, NSWindowDelegate {
    private var onClose: (() -> Void)?

    init(store: HostStore, onClose: @escaping () -> Void) {
        let rect = NSRect(x: 0, y: 0, width: 640, height: 540)
        let style: NSWindow.StyleMask = [.titled, .closable, .resizable]
        let window = NSWindow(contentRect: rect, styleMask: style, backing: .buffered, defer: false)
        window.title = "Settings"
        window.isReleasedWhenClosed = false
        self.onClose = onClose

        let view = SettingsView(store: store) { [weak window] in
            window?.close()
        }
        let hosting = NSHostingView(rootView: view)
        hosting.translatesAutoresizingMaskIntoConstraints = false

        super.init(window: window)
        window.delegate = self

        let container = NSView(frame: rect)
        container.addSubview(hosting)
        NSLayoutConstraint.activate([
            hosting.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            hosting.trailingAnchor.constraint(equalTo: container.trailingAnchor),
            hosting.topAnchor.constraint(equalTo: container.topAnchor),
            hosting.bottomAnchor.constraint(equalTo: container.bottomAnchor)
        ])
        window.contentView = container
        window.contentMinSize = NSSize(width: 540, height: 420)
        window.setContentSize(NSSize(width: 640, height: 540))
        window.center()
    }

    required init?(coder: NSCoder) { fatalError() }

    func windowWillClose(_ notification: Notification) {
        onClose?()
    }
}
