import AppKit

final class AddHostsWindowController: NSWindowController, NSWindowDelegate {
    private let textView = NSTextView()
    private let onConfirm: (Set<String>) -> Void
    private let onClose: () -> Void

    init(initialHosts: @escaping () -> [String], onConfirm: @escaping (Set<String>) -> Void, onClose: @escaping () -> Void) {
        self.onConfirm = onConfirm
        self.onClose = onClose
        let rect = NSRect(x: 0, y: 0, width: 540, height: 380)
        let style: NSWindow.StyleMask = [.titled, .closable, .resizable]
        let window = NSWindow(contentRect: rect, styleMask: style, backing: .buffered, defer: false)
        window.title = "Edit Hosts"
        window.isReleasedWhenClosed = false
        super.init(window: window)
        window.delegate = self
        configureContent(initialHosts: initialHosts)
    }

    required init?(coder: NSCoder) { fatalError() }

    private func configureContent(initialHosts: @escaping () -> [String]) {
        guard let contentView = window?.contentView else { return }

        let title = NSTextField(labelWithString: "Edit hosts")
        title.font = .systemFont(ofSize: 18, weight: .semibold)

        let info = NSTextField(labelWithString: "Enter one or more hosts. You can separate them with spaces, commas, or new lines.")
        info.lineBreakMode = .byWordWrapping

        textView.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        textView.isRichText = false
        textView.isAutomaticQuoteSubstitutionEnabled = false
        textView.isAutomaticDashSubstitutionEnabled = false
        textView.string = initialHosts().joined(separator: "\n")

        let scroll = NSScrollView()
        scroll.documentView = textView
        scroll.hasVerticalScroller = true
        scroll.translatesAutoresizingMaskIntoConstraints = false
        scroll.borderType = .bezelBorder

        let ok = NSButton(title: "OK", target: self, action: #selector(confirm))
        ok.keyEquivalent = "\r"
        let cancel = NSButton(title: "Cancel", target: self, action: #selector(cancel))
        cancel.keyEquivalent = "\u{1b}"

        let buttonStack = NSStackView(views: [cancel, ok])
        buttonStack.orientation = .horizontal
        buttonStack.spacing = 10
        buttonStack.alignment = .centerY

        let stack = NSStackView(views: [title, info, scroll, buttonStack])
        stack.orientation = .vertical
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 14, left: 14, bottom: 14, right: 14)
        stack.translatesAutoresizingMaskIntoConstraints = false

        contentView.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: contentView.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: contentView.trailingAnchor),
            stack.topAnchor.constraint(equalTo: contentView.topAnchor),
            stack.bottomAnchor.constraint(equalTo: contentView.bottomAnchor),
            scroll.heightAnchor.constraint(greaterThanOrEqualToConstant: 200)
        ])
    }

    @objc private func confirm() {
        onConfirm(parseHosts(textView.string))
        close()
    }

    @objc private func cancel() {
        close()
    }

    private func parseHosts(_ raw: String) -> Set<String> {
        let separators = CharacterSet.whitespacesAndNewlines.union(CharacterSet(charactersIn: ",;"))
        let tokens = raw
            .components(separatedBy: separators)
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
        return Set(tokens)
    }

    override func close() {
        super.close()
        onClose()
    }

    func windowWillClose(_ notification: Notification) {
        onClose()
    }
}
