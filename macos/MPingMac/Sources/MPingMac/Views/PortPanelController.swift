import AppKit

final class PortPanelController: NSWindowController, NSTextFieldDelegate, NSWindowDelegate {
    private let field = NSTextField()
    private let stepper = NSStepper()
    private var onConfirm: ((Int) -> Void)?
    private var onCancel: (() -> Void)?

    init(current: Int, onConfirm: @escaping (Int) -> Void, onCancel: @escaping () -> Void) {
        self.onConfirm = onConfirm
        self.onCancel = onCancel
        let contentRect = NSRect(x: 0, y: 0, width: 300, height: 160)
        let style: NSWindow.StyleMask = [.titled, .closable]
        let window = NSWindow(contentRect: contentRect, styleMask: style, backing: .buffered, defer: false)
        window.title = "Change TCP Port"
        window.isReleasedWhenClosed = false
        super.init(window: window)
        window.delegate = self

        let content = NSView(frame: contentRect)
        window.contentView = content

        let label = NSTextField(labelWithString: "Port (1–65535):")
        label.font = .systemFont(ofSize: 13, weight: .medium)

        field.alignment = .right
        let formatter = NumberFormatter()
        formatter.minimum = 1
        formatter.maximum = 65535
        formatter.allowsFloats = false
        field.formatter = formatter
        field.integerValue = current
        field.delegate = self
        field.focusRingType = .default

        stepper.minValue = 1
        stepper.maxValue = 65535
        stepper.increment = 1
        stepper.valueWraps = false
        stepper.integerValue = current
        stepper.target = self
        stepper.action = #selector(stepperChanged(_:))

        field.target = self
        field.action = #selector(fieldChanged(_:))

        let hStack = NSStackView(views: [field, stepper])
        hStack.orientation = .horizontal
        hStack.spacing = 8
        hStack.alignment = .centerY
        field.widthAnchor.constraint(equalToConstant: 90).isActive = true

        let ok = NSButton(title: "OK", target: self, action: #selector(confirm))
        ok.keyEquivalent = "\r"
        let cancel = NSButton(title: "Cancel", target: self, action: #selector(cancelAction))
        cancel.keyEquivalent = "\u{1b}"

        let buttonStack = NSStackView(views: [cancel, ok])
        buttonStack.orientation = .horizontal
        buttonStack.spacing = 8
        buttonStack.alignment = .centerY

        let vStack = NSStackView(views: [label, hStack, buttonStack])
        vStack.orientation = .vertical
        vStack.spacing = 12
        vStack.edgeInsets = NSEdgeInsets(top: 16, left: 16, bottom: 16, right: 16)
        vStack.translatesAutoresizingMaskIntoConstraints = false

        content.addSubview(vStack)
        NSLayoutConstraint.activate([
            vStack.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            vStack.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            vStack.topAnchor.constraint(equalTo: content.topAnchor),
            vStack.bottomAnchor.constraint(equalTo: content.bottomAnchor)
        ])
    }

    required init?(coder: NSCoder) { fatalError() }

    @objc private func stepperChanged(_ sender: NSStepper) {
        field.integerValue = sender.integerValue
    }

    @objc private func fieldChanged(_ sender: NSTextField) {
        stepper.integerValue = sender.integerValue
    }

    @objc private func confirm() {
        field.validateEditing()
        let value = field.integerValue
        guard (1...65535).contains(value) else { NSSound.beep(); return }
        onConfirm?(value)
        close()
    }

    @objc private func cancelAction() {
        onCancel?()
        close()
    }

    func controlTextDidChange(_ obj: Notification) {
        stepper.integerValue = field.integerValue
    }

    override func close() {
        super.close()
        onCancel?()
    }

    func windowWillClose(_ notification: Notification) {
        onCancel?()
    }
}
