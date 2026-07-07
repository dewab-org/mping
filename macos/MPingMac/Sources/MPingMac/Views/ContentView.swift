import SwiftUI
import AppKit
import UniformTypeIdentifiers

struct ContentView: View {
    @ObservedObject var store: HostStore
    @Binding var showStatusBar: Bool
    @State private var sortOrder = [KeyPathComparator(\Host.address)]
    @State private var visibleColumns: Set<ColumnKey>
    @State private var showHistory = true
    @State private var showingExportError = false
    @State private var portPanelController: PortPanelController?
    @State private var addHostsController: AddHostsWindowController?
    @State private var settingsController: SettingsWindowController?
    private let lastOKComparator = KeyPathComparator(\Host.lastOKSortValue)

    init(store: HostStore, showStatusBar: Binding<Bool>) {
        self.store = store
        self._showStatusBar = showStatusBar
        self._visibleColumns = State(initialValue: store.visibleColumns)
    }

    private var sortedHosts: [Host] {
        store.hosts.sorted(using: sortOrder)
    }

    private var visibleOrderedColumns: [ColumnKey] {
        ColumnKey.allCases.filter { visibleColumns.contains($0) }
    }

    private var selectedHost: Host? {
        store.hosts.first { store.selection.contains($0.id) }
    }

    var body: some View {
        VStack(spacing: 0) {
            header
            Divider()
            hostTable
                .tableStyle(.inset)
            if showHistory, let host = selectedHost {
                Divider()
                PingHistoryPanel(host: host, onClose: { showHistory = false }, onExport: { exportHistory(for: host) })
            }
            if showStatusBar {
                Divider()
                statusBar
            }
        }
        .background(
            Group {
                if #available(macOS 14, *) {
                    LinearGradient(colors: [Color(nsColor: .windowBackgroundColor), Color(nsColor: .controlBackgroundColor)], startPoint: .top, endPoint: .bottom)
                        .opacity(0.4)
                        .background(.ultraThinMaterial)
                } else {
                    LinearGradient(colors: [Color(nsColor: .windowBackgroundColor), Color(nsColor: .controlBackgroundColor)], startPoint: .top, endPoint: .bottom)
                        .opacity(0.7)
                }
            }
        )
        .onReceive(NotificationCenter.default.publisher(for: .commandEditHosts)) { _ in
            presentEditHosts()
        }
        .onReceive(NotificationCenter.default.publisher(for: .commandResetHosts)) { _ in
            handleResetSelection()
        }
        .onReceive(NotificationCenter.default.publisher(for: .commandDeleteHosts)) { _ in
            handleDeleteSelection()
        }
        .onReceive(NotificationCenter.default.publisher(for: .commandEditPort)) { _ in
            handleEditPortCommand()
        }
        .toolbar {
            ToolbarItemGroup {
                Button {
                    var newSettings = store.settings
                    newSettings.notificationsEnabled.toggle()
                    store.applySettings(newSettings)
                } label: {
                    Label("Alerts", systemImage: store.settings.notificationsEnabled ? "bell.fill" : "bell.slash")
                        .labelStyle(.titleAndIcon)
                        .symbolVariant(store.settings.toolbarIconStyle.symbolVariant ?? .none)
                }
                .help(store.settings.notificationsEnabled ? "Disable alert sounds" : "Enable alert sounds")
            }
        }
        .frame(minWidth: 900, minHeight: 520)
        .alert("Export failed", isPresented: $showingExportError) {
            Button("OK", role: .cancel) {}
        } message: {
            Text("Could not write the CSV file.")
        }
    }

    private var header: some View {
        HStack {
            VStack(alignment: .leading, spacing: 4) {
                Text("Multi-host ping monitor")
                    .font(.title2.weight(.semibold))
            }
            Spacer()
            HStack(spacing: 10) {
                Button {
                    presentSettings()
                } label: {
                    Label("Settings", systemImage: "gearshape")
                        .labelStyle(.titleAndIcon)
                }
                Button {
                    presentEditHosts()
                } label: {
                    Label("Edit Hosts", systemImage: "square.and.pencil")
                        .labelStyle(.titleAndIcon)
                }
            }
            .buttonStyle(.borderedProminent)
        }
        .padding(.horizontal)
        .padding(.vertical, 12)
    }

    private func rttLabel(for host: Host) -> String {
        guard let rtt = host.lastRTT else { return "—" }
        return String(format: "%.2f ms", rtt * 1000)
    }

    private var statusBar: some View {
        HStack {
            Label("\(store.hosts.count) hosts", systemImage: "list.dash")
            Spacer()
            Button {
                var updated = store.settings
                updated.notificationsEnabled.toggle()
                store.applySettings(updated)
            } label: {
                Label(store.settings.notificationsEnabled ? "Alerts on" : "Alerts off", systemImage: store.settings.notificationsEnabled ? "bell.fill" : "bell.slash")
            }
            .buttonStyle(.link)
            .foregroundStyle(.secondary)
            Button(showHistory ? "Hide History" : "Show History") {
                showHistory.toggle()
            }
            .buttonStyle(.link)
            if let selected = store.hosts.first(where: { store.selection.contains($0.id) }) {
                Text("Selected: \(selected.displayName)")
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 6)
        .background(.thickMaterial)
    }

    private var hostTable: some View {
        Group {
            if #available(macOS 14.4, *) {
                modernTable
            } else {
                legacyTable
            }
        }
        .transaction { $0.disablesAnimations = true }
        .animation(.none, value: store.hosts.map(\.id))
        .contextMenu(forSelectionType: Host.ID.self) { selection in
            contextMenuContent(for: selection)
        }
        .onChange(of: sortOrder) { newValue in
            if newValue.contains(lastOKComparator) {
                sortOrder = newValue.filter { $0 != lastOKComparator }
            }
        }
        .onChange(of: visibleColumns) { newValue in
            store.visibleColumns = newValue
        }
        .onChange(of: store.visibleColumns) { newValue in
            if newValue != visibleColumns {
                visibleColumns = newValue
            }
        }
        .overlay(alignment: .topLeading) {
            Color.clear
                .frame(height: 28)
                .contentShape(Rectangle())
                .contextMenu {
                    ForEach(ColumnKey.allCases, id: \.self) { key in
                        Toggle(isOn: binding(for: key)) { Text(columnLabel(for: key)) }
                    }
                }
        }
    }

    @available(macOS 14.4, *)
    private var modernTable: some View {
        Table(sortedHosts, selection: $store.selection, sortOrder: $sortOrder) {
            TableColumnForEach(visibleOrderedColumns, id: \.self) { key in
                column(for: key)
            }
        }
    }

    private var legacyTable: some View {
        Table(sortedHosts, selection: $store.selection, sortOrder: $sortOrder) {
            Group {
                TableColumn(isVisible(.status) ? "Status" : "", value: \Host.statusSortValue) { host in AnyView(rowContextMenu(for: host) { statusIndicator(for: host) }.opacity(isVisible(.status) ? 1 : 0)) }
                TableColumn(isVisible(.host) ? "Host" : "", value: \Host.address) { host in AnyView(hostCell(for: host).opacity(isVisible(.host) ? 1 : 0)) }
                TableColumn(isVisible(.ip) ? "IP" : "", value: \Host.ipSortValue) { host in AnyView(rowContextMenu(for: host) { ipCell(for: host) }.opacity(isVisible(.ip) ? 1 : 0)) }
                TableColumn(isVisible(.rtt) ? "RTT" : "", value: \Host.rttSortValue) { host in AnyView(rowContextMenu(for: host) { rttCell(for: host) }.opacity(isVisible(.rtt) ? 1 : 0)) }
                TableColumn(isVisible(.rttTrend) ? "RTT Trend" : "", value: \Host.rttSortValue) { host in AnyView(rowContextMenu(for: host) { SparklineView(values: host.rttHistory, color: .blue) }.opacity(isVisible(.rttTrend) ? 1 : 0)) }
            }
            Group {
                TableColumn(isVisible(.successTrend) ? "Success Trend" : "", value: \Host.successRatePercent) { host in AnyView(rowContextMenu(for: host) { SparklineView(values: host.successHistory, rangeOverride: 0...100, gradientRange: 0...100, fill: true) }.opacity(isVisible(.successTrend) ? 1 : 0)) }
                TableColumn(isVisible(.success) ? "Success" : "", value: \Host.successRatePercent) { host in AnyView(rowContextMenu(for: host) { VStack(alignment: .leading) { Text("\(host.successCount)").font(.body.monospacedDigit()); Text("\(Int(host.successRate * 100))%").font(.caption.monospacedDigit()).foregroundStyle(.secondary) } }.opacity(isVisible(.success) ? 1 : 0)) }
                TableColumn(isVisible(.failure) ? "Fail" : "", value: \Host.failureCount) { host in AnyView(rowContextMenu(for: host) { Text("\(host.failureCount)").font(.body.monospacedDigit()) }.opacity(isVisible(.failure) ? 1 : 0)) }
                TableColumn(isVisible(.lastOK) ? "Last OK" : "", value: \Host.lastOKSortValue) { host in AnyView(rowContextMenu(for: host) { Text(store.lastOKLabel(for: host)).font(.body).foregroundStyle(host.lastOK == nil ? .secondary : .primary) }.opacity(isVisible(.lastOK) ? 1 : 0)) }
                TableColumn(isVisible(.error) ? "Error" : "", value: \Host.errorSortValue) { host in AnyView(rowContextMenu(for: host) { Text(host.lastError ?? "—").foregroundColor(host.lastError == nil ? .secondary : .red).lineLimit(1).help(host.lastError ?? "") }.opacity(isVisible(.error) ? 1 : 0)) }
            }
        }
    }

    private func column(for key: ColumnKey) -> TableColumn<Host, KeyPathComparator<Host>, AnyView, Text> {
        switch key {
        case .status:
            TableColumn("Status", value: \.statusSortValue) { host in AnyView(rowContextMenu(for: host) { statusIndicator(for: host) }) }
                .width(min: 120, ideal: 140)
        case .host:
            TableColumn("Host", value: \.address) { host in AnyView(hostCell(for: host)) }
        case .ip:
            TableColumn("IP", value: \.ipSortValue) { host in AnyView(rowContextMenu(for: host) { ipCell(for: host) }) }
                .width(min: 140, ideal: 170)
        case .rtt:
            TableColumn("RTT", value: \.rttSortValue) { host in AnyView(rowContextMenu(for: host) { rttCell(for: host) }) }
                .width(min: 80, ideal: 100)
        case .rttMin:
            TableColumn("RTT Min", value: \.minRTTSortValue) { host in AnyView(rowContextMenu(for: host) { Text(host.minRTT.map { String(format: "%.1f", $0 * 1000) } ?? "–").font(.body.monospacedDigit()) }) }
                .width(min: 80, ideal: 100)
        case .rttAvg:
            TableColumn("RTT Avg", value: \.avgRTTSortValue) { host in AnyView(rowContextMenu(for: host) { Text(host.averageRTT.map { String(format: "%.1f", $0 * 1000) } ?? "–").font(.body.monospacedDigit()) }) }
                .width(min: 80, ideal: 100)
        case .rttMax:
            TableColumn("RTT Max", value: \.maxRTTSortValue) { host in AnyView(rowContextMenu(for: host) { Text(host.maxRTT.map { String(format: "%.1f", $0 * 1000) } ?? "–").font(.body.monospacedDigit()) }) }
                .width(min: 80, ideal: 100)
        case .rttTrend:
            TableColumn("RTT Trend", value: \.rttSortValue) { host in AnyView(rowContextMenu(for: host) { SparklineView(values: host.rttHistory, color: .blue) }) }
                .width(min: 120, ideal: 150)
        case .successTrend:
            TableColumn("Success Trend", value: \.successRatePercent) { host in
                AnyView(rowContextMenu(for: host) {
                    SparklineView(values: host.successHistory, rangeOverride: 0...100, gradientRange: 0...100, fill: true)
                })
            }
            .width(min: 120, ideal: 150)
        case .success:
            TableColumn("Success", value: \.successRatePercent) { host in
                AnyView(rowContextMenu(for: host) {
                    VStack(alignment: .leading) {
                        Text("\(host.successCount)").font(.body.monospacedDigit())
                        Text("\(Int(host.successRate * 100))%").font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                    }
                })
            }
            .width(min: 80, ideal: 100)
        case .failure:
            TableColumn("Fail", value: \.failureCount) { host in AnyView(rowContextMenu(for: host) { Text("\(host.failureCount)").font(.body.monospacedDigit()) }) }
                .width(min: 70, ideal: 90)
        case .lastOK:
            TableColumn("Last OK", value: \.lastOKSortValue) { host in AnyView(rowContextMenu(for: host) { Text(store.lastOKLabel(for: host)).font(.body).foregroundStyle(host.lastOK == nil ? .secondary : .primary) }) }
                .width(min: 120, ideal: 150)
        case .loss:
            TableColumn("Loss %", value: \.lossSortValue) { host in AnyView(rowContextMenu(for: host) { Text("\(Int(host.lossRate * 100))%").font(.body.monospacedDigit()) }) }
                .width(min: 70, ideal: 90)
        case .error:
            TableColumn("Error", value: \.errorSortValue) { host in
                AnyView(rowContextMenu(for: host) {
                    Text(host.lastError ?? "—")
                        .foregroundColor(host.lastError == nil ? .secondary : .red)
                        .lineLimit(1)
                        .help(host.lastError ?? "")
                })
            }
            .width(min: 180, ideal: 220)
        }
    }

    private func hostCell(for host: Host) -> some View {
        VStack(alignment: .leading) {
            Text(host.displayName)
                .font(.headline)
                .lineLimit(1)
                .layoutPriority(1)
            if host.displayName != host.address {
                Text(host.address)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
                    .layoutPriority(1)
            }
        }
        .accessibilityLabel(host.displayName)
    }

    private func ipCell(for host: Host) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(host.resolvedIP ?? "—")
                .font(.body.monospacedDigit())
                .lineLimit(1)
                .layoutPriority(1)
            if host.backend == .tcp {
                let port = host.tcpPortOverride ?? store.settings.defaultTCPPort
                Text("TCP :\(port)")
                    .font(.caption2.monospacedDigit())
                    .foregroundStyle(.secondary)
            }
        }
    }


    private func rttCell(for host: Host) -> some View {
        Text(rttLabel(for: host))
            .font(.body.monospacedDigit())
    }

    private func exportHistory(for host: Host) {
        guard !host.samples.isEmpty else { return }
        let panel = NSSavePanel()
        panel.nameFieldStringValue = "\(host.displayName)-history.csv"
        panel.allowedContentTypes = [.commaSeparatedText]
        panel.canCreateDirectories = true
        panel.begin { response in
            guard response == .OK, let url = panel.url else { return }
            Task.detached {
                let header = "timestamp,success,rtt_ms,error\n"
                let formatter = ISO8601DateFormatter()
                let rows = host.samples.map { sample in
                    let ts = formatter.string(from: sample.timestamp)
                    let success = sample.success ? "1" : "0"
                    let rtt = sample.rttMilliseconds.map { String(format: "%.1f", $0) } ?? ""
                    let error = sample.error?.replacingOccurrences(of: "\"", with: "\"\"") ?? ""
                    return "\"\(ts)\",\(success),\(rtt),\"\(error)\""
                }
                let csv = header + rows.joined(separator: "\n")
                do {
                    try csv.write(to: url, atomically: true, encoding: .utf8)
                } catch {
                    DispatchQueue.main.async {
                        showingExportError = true
                    }
                }
            }
        }
    }

    private func isVisible(_ key: ColumnKey) -> Bool {
        visibleColumns.contains(key)
    }

    private func binding(for key: ColumnKey) -> Binding<Bool> {
        Binding(
            get: { visibleColumns.contains(key) },
            set: { newValue in
                if newValue {
                    visibleColumns.insert(key)
                } else {
                    visibleColumns.remove(key)
                }
            }
        )
    }

    private func columnLabel(for key: ColumnKey) -> String {
        switch key {
        case .status: return "Status"
        case .host: return "Host"
        case .ip: return "IP"
        case .rtt: return "RTT"
        case .rttTrend: return "RTT Trend"
        case .successTrend: return "Success Trend"
        case .success: return "Success"
        case .failure: return "Fail"
        case .lastOK: return "Last OK"
        case .error: return "Error"
        case .rttMin: return "RTT Min"
        case .rttAvg: return "RTT Avg"
        case .rttMax: return "RTT Max"
        case .loss: return "Loss %"
        }
    }

    private func statusIndicator(for host: Host) -> some View {
        let success = host.lastSuccess
        let systemImage = success == true ? "arrow.up.circle.fill" : "arrow.down.circle.fill"
        let color: Color = success == true ? .green : .red

        return HStack(spacing: 6) {
            if host.paused {
                Image(systemName: "pause.circle.fill")
                    .foregroundColor(.yellow)
                    .help("Host is paused")
            }
            Image(systemName: systemImage)
                .foregroundColor(success == nil ? .secondary : color)
                .help(success == true ? "Last ping succeeded" : success == false ? "Last ping failed" : "No pings yet")
            Text(host.backend == .tcp ? "TCP" : "ICMP")
                .font(.caption2.bold())
                .foregroundColor(.white)
                .padding(.horizontal, 6)
                .padding(.vertical, 2)
                .background(Color.blue.opacity(0.8))
                .clipShape(Capsule())
                .fixedSize()
            Spacer()
        }
    }

    private var timeoutControls: some View { EmptyView() }

    @ViewBuilder
    private func rowContextMenu<Content: View>(for host: Host, @ViewBuilder content: () -> Content) -> some View {
        content()
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.vertical, host.paused ? 2 : 0)
            .background(host.paused ? Color.yellow.opacity(0.15) : Color.clear)
    }

    @ViewBuilder
    private func contextMenuContent(for selection: Set<Host.ID>?) -> some View {
        let ids = menuSelection(from: selection)
        let hosts = hosts(for: ids)
        let allTCP = !hosts.isEmpty && hosts.allSatisfy { $0.backend == .tcp }
        let allPaused = !hosts.isEmpty && hosts.allSatisfy { $0.paused }

        Button("Reset Stats (⌘R)") {
            store.resetStats(for: ids)
        }
        .disabled(ids.isEmpty)

        Menu("Protocol") {
            Button {
                store.updateBackend(for: ids, backend: .swiftNative)
            } label: {
                Label("ICMP", systemImage: hosts.allSatisfy { $0.backend == .swiftNative } ? "checkmark" : "")
            }
            Button {
                store.updateBackend(for: ids, backend: .tcp)
            } label: {
                Label("TCP", systemImage: hosts.allSatisfy { $0.backend == .tcp } ? "checkmark" : "")
            }
        }
        .disabled(ids.isEmpty)

        Button("Change TCP Port… (⌘P)") {
            if let first = hosts.first {
                presentTCPPortPrompt(for: first)
            }
        }
        .disabled(!allTCP)

        Button(allPaused ? "Resume" : "Pause") {
            ids.forEach { store.togglePause($0) }
        }
        .disabled(ids.isEmpty)

        Divider()

        Button("Delete Host (⌘⌫)", role: .destructive) {
            store.deleteHosts(with: ids)
            store.selection.subtract(ids)
        }
        .disabled(ids.isEmpty)
    }

    private func menuSelection(from selection: Set<Host.ID>?) -> Set<Host.ID> {
        if let selection, !selection.isEmpty { return selection }
        if !store.selection.isEmpty { return store.selection }
        if let first = selectedHost?.id { return [first] }
        return []
    }

    private func hosts(for ids: Set<Host.ID>) -> [Host] {
        guard !ids.isEmpty else { return [] }
        return store.hosts.filter { ids.contains($0.id) }
    }

    private func presentEditHosts() {
        if addHostsController == nil {
            addHostsController = AddHostsWindowController(
                initialHosts: { store.hosts.map(\.inputRepresentation) },
                onConfirm: { hosts in
                    store.syncHosts(to: hosts)
                },
                onClose: { addHostsController = nil }
            )
        }
        addHostsController?.showWindow(nil)
        addHostsController?.window?.center()
        addHostsController?.window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    private func handleResetSelection() {
        let ids = menuSelection(from: store.selection)
        guard !ids.isEmpty else { return }
        store.resetStats(for: ids)
    }

    private func handleDeleteSelection() {
        let ids = menuSelection(from: store.selection)
        guard !ids.isEmpty else { return }
        store.deleteHosts(with: ids)
        store.selection.subtract(ids)
    }

    private func handleEditPortCommand() {
        let ids = menuSelection(from: store.selection)
        guard let firstID = ids.first, let host = store.hosts.first(where: { $0.id == firstID }) else {
            NSSound.beep()
            return
        }
        guard host.backend == .tcp else {
            NSSound.beep()
            return
        }
        presentTCPPortPrompt(for: host)
    }

    private func presentTCPPortPrompt(for host: Host) {
        let currentPort = host.tcpPortOverride ?? store.settings.defaultTCPPort
        let targets = store.selection.isEmpty ? [host.id] : Array(store.selection)

        let controller = PortPanelController(current: currentPort) { value in
            store.updateTCPPort(for: Set(targets), port: value)
            portPanelController = nil
        } onCancel: {
            portPanelController = nil
        }
        portPanelController = controller
        controller.showWindow(nil)
        if let window = controller.window {
            if let parent = NSApp.keyWindow {
                window.center()
                parent.addChildWindow(window, ordered: .above)
            } else {
                window.center()
            }
            window.makeKeyAndOrderFront(nil)
        }
        NSApp.activate(ignoringOtherApps: true)
    }

    private func presentSettings() {
        if settingsController == nil {
            settingsController = SettingsWindowController(store: store) { settingsController = nil }
        }
        settingsController?.showWindow(nil)
        settingsController?.window?.center()
        settingsController?.window?.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }
}
