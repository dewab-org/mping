import SwiftUI

struct SettingsView: View {
    @ObservedObject var store: HostStore
    var onClose: (() -> Void)?

    @State private var draft: MonitoringSettings

    init(store: HostStore, onClose: (() -> Void)? = nil) {
        self._store = ObservedObject(wrappedValue: store)
        self.onClose = onClose
        self._draft = State(initialValue: store.settings)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Settings")
                .font(.title2.weight(.semibold))
                .padding(.bottom, 4)

            Form {
                Section("TCP & Packet") {
                    HStack {
                        Text("Default TCP port")
                        Spacer()
                        valueFieldInt(binding: Binding(
                            get: { draft.defaultTCPPort },
                            set: { draft.defaultTCPPort = min(max($0, 1), 65535) }
                        ), range: 1...65535)
                    }
                    HStack {
                        Text("Payload size")
                        Spacer()
                        valueFieldInt(binding: Binding(
                            get: { draft.pingPayloadBytes },
                            set: { draft.pingPayloadBytes = max(0, min(2048, $0)) }
                        ), range: 0...2048)
                        Text("bytes")
                            .foregroundStyle(.secondary)
                    }
                    Toggle("Do Not Fragment", isOn: $draft.doNotFragment)
                    HStack {
                        Text("Time To Live")
                        Spacer()
                        valueFieldInt(binding: Binding(
                            get: { draft.ttl },
                            set: { draft.ttl = min(max($0, 1), 255) }
                        ), range: 1...255)
                        Text("hops")
                            .foregroundStyle(.secondary)
                    }
                }

                Section("Timing") {
                    HStack {
                        Text("Interval")
                        Spacer()
                        valueFieldDouble(binding: $draft.intervalSeconds, range: 0.5...300, step: 0.1, suffix: "s")
                        Stepper("", value: $draft.intervalSeconds, in: 0.5...300, step: 0.5)
                            .labelsHidden()
                    }
                    HStack {
                        Text("Timeout")
                        Spacer()
                        valueFieldDouble(binding: $draft.timeoutSeconds, range: 0.5...60, step: 0.1, suffix: "s")
                        Stepper("", value: $draft.timeoutSeconds, in: 0.5...60, step: 0.5)
                            .labelsHidden()
                    }
                    HStack {
                        Text("Alert cooldown")
                        Spacer()
                        valueFieldDouble(binding: $draft.notificationCooldownSeconds, range: 5...300, step: 1, suffix: "s")
                        Stepper("", value: $draft.notificationCooldownSeconds, in: 5...300, step: 5)
                            .labelsHidden()
                        .disabled(!draft.notificationsEnabled)
                    }
                }

                Section("Concurrency") {
                    HStack {
                        Text("Max concurrent pings")
                        Spacer()
                        valueFieldInt(binding: $draft.maxConcurrentPings, range: 1...256)
                        Stepper("", value: $draft.maxConcurrentPings, in: 1...256, step: 1)
                            .labelsHidden()
                    }
                }

                Section("Notifications") {
                    Toggle("Enable alert sounds", isOn: $draft.notificationsEnabled)
                    Toggle("Show notification banners", isOn: $draft.popupNotificationsEnabled)
                    Picker("Failure sound", selection: $draft.failureSoundName) {
                        ForEach(SoundLibrary.availableNames, id: \.self) { name in
                            Text(name).tag(name)
                        }
                    }
                    .disabled(!draft.notificationsEnabled)
                    Picker("Recovery sound", selection: $draft.clearSoundName) {
                        ForEach(SoundLibrary.availableNames, id: \.self) { name in
                            Text(name).tag(name)
                        }
                    }
                    .disabled(!draft.notificationsEnabled)
                }

            }
            .formStyle(.grouped)

            HStack {
                Button("Restore Defaults") {
                    draft = .default
                }
                Spacer()
                Button("Cancel") {
                    onClose?()
                }
                .keyboardShortcut(.cancelAction)
                Button("Save") {
                    store.applySettings(draft)
                    onClose?()
                }
                .keyboardShortcut(.defaultAction)
            }
        }
        .padding()
        .frame(minWidth: 540, idealWidth: 620, maxWidth: 760, minHeight: 420)
        .onChange(of: store.settings) { draft = $0 }
        .onChange(of: draft.failureSoundName) { _ in playPreview(name: draft.failureSoundName) }
        .onChange(of: draft.clearSoundName) { _ in playPreview(name: draft.clearSoundName) }
        .onChange(of: draft.popupNotificationsEnabled) { enabled in
            if enabled {
                NotificationHelper.requestAuthorization()
            }
        }
    }

    private func valueFieldDouble(binding: Binding<Double>, range: ClosedRange<Double>, step: Double, suffix: String) -> some View {
        HStack(spacing: 4) {
            TextField("", value: binding, formatter: numberFormatter)
                .multilineTextAlignment(.trailing)
                .frame(width: 80)
            Text(suffix)
                .foregroundStyle(.secondary)
        }
        .onChange(of: binding.wrappedValue) { newValue in
            var clamped = newValue
            if clamped < range.lowerBound { clamped = range.lowerBound }
            if clamped > range.upperBound { clamped = range.upperBound }
            if abs(clamped - newValue) > .ulpOfOne {
                binding.wrappedValue = clamped
            }
        }
    }

    private func valueFieldInt(binding: Binding<Int>, range: ClosedRange<Int>) -> some View {
        HStack(spacing: 4) {
            TextField("", value: binding, formatter: intFormatter)
                .multilineTextAlignment(.trailing)
                .frame(width: 80)
        }
        .onChange(of: binding.wrappedValue) { newValue in
            var clamped = newValue
            if clamped < range.lowerBound { clamped = range.lowerBound }
            if clamped > range.upperBound { clamped = range.upperBound }
            if clamped != newValue {
                binding.wrappedValue = clamped
            }
        }
    }

    private var numberFormatter: NumberFormatter {
        let formatter = NumberFormatter()
        formatter.numberStyle = .decimal
        formatter.maximumFractionDigits = 2
        return formatter
    }

    private var intFormatter: NumberFormatter {
        let formatter = NumberFormatter()
        formatter.numberStyle = .none
        return formatter
    }

    private func playPreview(name: String) {
        let sound = NSSound(named: NSSound.Name(name))
        sound?.play()
    }
}
