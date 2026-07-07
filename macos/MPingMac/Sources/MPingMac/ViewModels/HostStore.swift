import Foundation
import SwiftUI
import AppKit

@MainActor
final class HostStore: ObservableObject {
    @Published private(set) var hosts: [Host] = []
    @Published var selection: Set<Host.ID> = []
    @Published var settings: MonitoringSettings
    @Published var visibleColumns: Set<ColumnKey> = Set(ColumnKey.allCases) {
        didSet { schedulePersist() }
    }
    @Published private var relativeNow: Date = .now

    nonisolated let swiftBackend: PingBackend
    nonisolated let tcpBackend: PingBackend
    private let gate: ConcurrencyGate
    private var tasks: [Host.ID: Task<Void, Never>] = [:]
    private let relativeFormatter = RelativeDateTimeFormatter()
    private var relativeTimer: Timer?
    private let persistenceURL: URL
    private var lastFailureNotification: Date?
    private var lastSuccessNotification: Date?
    private var reverseCache: [String: String] = [:]
    private var reverseInFlight: Set<String> = []
    private let soundQueue = SoundQueue()
    private var persistTask: Task<Void, Never>?
    private var loopHeartbeats: [Host.ID: Date] = [:]

    init(settings: MonitoringSettings = .default) {
        self.settings = settings
        self.swiftBackend = SwiftICMPBackend()
        self.tcpBackend = TCPPingBackend()
        self.gate = ConcurrencyGate(limit: settings.maxConcurrentPings)
        let supportDir = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first ?? URL(fileURLWithPath: NSTemporaryDirectory())
        self.persistenceURL = supportDir.appendingPathComponent("mping/hosts.json", isDirectory: false)
        loadPersisted()
        startRelativeTimer()
    }

    func addHosts(from rawInput: String) -> AddHostResult {
        let cleaned = Self.normalizeHosts(rawInput)
        var added: [Host] = []
        var duplicates: [String] = []

        for entry in cleaned {
            if hosts.contains(where: { $0.matches(address: entry.address, tcpPort: entry.tcpPortOverride, backend: entry.backend) }) {
                duplicates.append(entry.displayString)
                continue
            }

            let host = Host(
                address: entry.address,
                resolvedIP: nil,
                resolvedName: nil,
                lastRTT: nil,
                lastError: nil,
                lastOK: nil,
                successCount: 0,
                failureCount: 0,
                lastSuccess: nil,
                rttHistory: [],
                successHistory: [],
                samples: [],
                interval: settings.intervalSeconds,
                timeout: settings.timeoutSeconds,
                paused: false,
                backend: entry.backend ?? .swiftNative,
                tcpPortOverride: entry.tcpPortOverride,
                minRTT: nil,
                maxRTT: nil,
                totalRTT: 0,
                rttSampleCount: 0
            )
            hosts.append(host)
            added.append(host)
            startLoop(for: host.id)
        }

        if !added.isEmpty { schedulePersist() }
        return AddHostResult(added: added, duplicates: duplicates)
    }

    func deleteSelection() {
        deleteHosts(with: selection)
        selection.removeAll()
    }

    func togglePause(_ id: Host.ID) {
        guard let idx = hosts.firstIndex(where: { $0.id == id }) else { return }
        hosts[idx].paused.toggle()
        if !hosts[idx].paused {
            startLoop(for: id)
        }
        schedulePersist()
    }

    func updateTCPPort(for ids: Set<Host.ID>, port: Int) {
        guard (1...65535).contains(port) else { return }
        guard !ids.isEmpty else { return }
        for index in hosts.indices where ids.contains(hosts[index].id) {
            hosts[index].tcpPortOverride = port
        }
        schedulePersist()
    }

    func updateBackend(for ids: Set<Host.ID>, backend: PingBackendKind) {
        guard !ids.isEmpty else { return }
        for index in hosts.indices where ids.contains(hosts[index].id) {
            hosts[index].backend = backend
        }
        schedulePersist()
    }

    func deleteHosts(with ids: Set<Host.ID>) {
        hosts.removeAll { host in
            if ids.contains(host.id) {
                tasks[host.id]?.cancel()
                tasks[host.id] = nil
                return true
            }
            return false
        }
        schedulePersist()
    }

    func resetStats(for ids: Set<Host.ID>) {
        guard !ids.isEmpty else { return }
        for index in hosts.indices {
            if ids.contains(hosts[index].id) {
                hosts[index].successCount = 0
                hosts[index].failureCount = 0
                hosts[index].lastRTT = nil
                hosts[index].lastError = nil
                hosts[index].lastOK = nil
                hosts[index].lastSuccess = nil
                hosts[index].rttHistory.removeAll()
                hosts[index].successHistory.removeAll()
            }
        }
        schedulePersist()
    }

    func syncHosts(to addresses: Set<String>) {
        let normalized = Self.normalizeHostTokens(Array(addresses))
        let inputByKey = Dictionary(uniqueKeysWithValues: normalized.map { (Self.canonicalKey(for: $0), $0) })
        let inputKeys = Set(inputByKey.keys)
        let existingKeys = Set(hosts.map { Self.canonicalKey(address: $0.address, port: $0.tcpPortOverride, backend: $0.backend) })

        let toRemove = existingKeys.subtracting(inputKeys)
        if !toRemove.isEmpty {
            let ids = hosts.filter { toRemove.contains(Self.canonicalKey(address: $0.address, port: $0.tcpPortOverride, backend: $0.backend)) }.map(\.id)
            deleteHosts(with: Set(ids))
        }

        let toAdd = inputKeys.subtracting(existingKeys)
        guard !toAdd.isEmpty else { return }

        for key in toAdd {
            guard let entry = inputByKey[key] else { continue }
            let host = Host(
                address: entry.address,
                resolvedIP: nil,
                resolvedName: nil,
                lastRTT: nil,
                lastError: nil,
                lastOK: nil,
                successCount: 0,
                failureCount: 0,
                lastSuccess: nil,
                rttHistory: [],
                successHistory: [],
                samples: [],
                interval: settings.intervalSeconds,
                timeout: settings.timeoutSeconds,
                paused: false,
                backend: entry.backend ?? .swiftNative,
                tcpPortOverride: entry.tcpPortOverride,
                minRTT: nil,
                maxRTT: nil,
                totalRTT: 0,
                rttSampleCount: 0
            )
            hosts.append(host)
            startLoop(for: host.id)
        }
        schedulePersist()
    }

    func applySettings(_ newSettings: MonitoringSettings) {
        guard newSettings != settings else { return }
        if newSettings.popupNotificationsEnabled && !settings.popupNotificationsEnabled {
            NotificationHelper.requestAuthorization()
        }
        settings = newSettings

        for index in hosts.indices {
            hosts[index].interval = newSettings.intervalSeconds
            hosts[index].timeout = newSettings.timeoutSeconds
        }

        Task {
            await gate.setLimit(newSettings.maxConcurrentPings)
        }
        schedulePersist()
    }
    
    func adjustInterval(by delta: Double) {
        var updated = settings
        updated.intervalSeconds = max(0.5, settings.intervalSeconds + delta)
        applySettings(updated)
    }

    func adjustTimeout(by delta: Double) {
        var updated = settings
        updated.timeoutSeconds = max(0.5, settings.timeoutSeconds + delta)
        applySettings(updated)
    }

    func lastOKLabel(for host: Host) -> String {
        guard let lastOK = host.lastOK else { return "—" }
        let delta = relativeNow.timeIntervalSince(lastOK)
        if delta < 1 {
            return "just now"
        }
        return relativeFormatter.localizedString(for: lastOK, relativeTo: relativeNow)
    }

    func shutdown() {
        tasks.values.forEach { $0.cancel() }
        tasks.removeAll()
        relativeTimer?.invalidate()
    }

    func clearHosts() {
        tasks.values.forEach { $0.cancel() }
        tasks.removeAll()
        hosts.removeAll()
        selection.removeAll()
        schedulePersist()
    }

    private func startLoop(for hostID: Host.ID) {
        tasks[hostID]?.cancel()
        tasks[hostID] = Task.detached(priority: .utility) { [weak self] in
            guard let store = self else { return }
            while !Task.isCancelled {
                await MainActor.run {
                    store.recordHeartbeat(for: hostID)
                }
                guard let params = await store.hostParameters(for: hostID) else { return }
                if params.paused {
                    try? await Task.sleep(nanoseconds: UInt64(max(params.interval, 0.5) * 1_000_000_000))
                    continue
                }
                let settings = await MainActor.run { store.settings }
                await store.gate.acquire()
                let defaultPort = settings.defaultTCPPort
                let payload = settings.pingPayloadBytes
                let ttl = settings.ttl
                let doNotFragment = settings.doNotFragment
                let backend: PingBackend = params.backend == .tcp ? store.tcpBackend : store.swiftBackend
                let result = await backend.ping(
                    host: params.address,
                    timeout: params.timeout,
                    payloadSize: payload,
                    ttl: ttl,
                    doNotFragment: doNotFragment,
                    tcpPort: params.tcpPort ?? defaultPort
                )
                await store.gate.release()

                await store.apply(result: result, to: hostID)
                let interval = max(params.interval, 0.5)
                try? await Task.sleep(nanoseconds: UInt64(interval * 1_000_000_000))
            }
        }
    }

    private func hostParameters(for id: Host.ID) -> (address: String, interval: TimeInterval, timeout: TimeInterval, paused: Bool, tcpPort: Int?, backend: PingBackendKind)? {
        guard let host = hosts.first(where: { $0.id == id }) else { return nil }
        return (host.address, host.interval, host.timeout, host.paused, host.tcpPortOverride, host.backend)
    }

    private func apply(result: PingResult, to hostID: Host.ID) {
        guard let index = hosts.firstIndex(where: { $0.id == hostID }) else { return }
        var host = hosts[index]
        let previousSuccess = host.lastSuccess
        var changed = false

        if result.success {
            host.successCount += 1
            host.lastOK = Date()
            host.lastError = nil
            host.lastSuccess = true
            maybeNotifyTransition(previousSuccess: previousSuccess, nowSuccess: true)
        } else {
            host.failureCount += 1
            host.lastError = result.errorDescription ?? "Unknown error"
            host.lastSuccess = false
            maybeNotifyTransition(previousSuccess: previousSuccess, nowSuccess: false)
        }
        changed = true

        if let resolvedName = result.resolvedName, !resolvedName.isEmpty, resolvedName != host.resolvedName {
            host.resolvedName = resolvedName
            changed = true
        }
        if let resolvedIP = result.resolvedIP, !resolvedIP.isEmpty, resolvedIP != host.resolvedIP {
            host.resolvedIP = resolvedIP
            changed = true
            if host.resolvedName == host.address || host.resolvedName == nil {
                Task.detached { [weak self] in
                    guard let self else { return }
                    if let name = await self.reverseLookup(ip: resolvedIP) {
                        await MainActor.run {
                            if let idx = self.hosts.firstIndex(where: { $0.id == hostID }) {
                                self.hosts[idx].resolvedName = name
                            }
                        }
                    }
                }
            }
        }
        host.lastRTT = result.rtt
        if let rtt = result.rtt {
            let ms = rtt * 1000
            host.rttHistory.append(ms)
            if host.rttHistory.count > 40 {
                host.rttHistory.removeFirst(host.rttHistory.count - 40)
            }
            changed = true
            host.rttSampleCount += 1
            host.totalRTT += rtt
            if let min = host.minRTT {
                host.minRTT = Swift.min(min, rtt)
            } else {
                host.minRTT = rtt
            }
            if let max = host.maxRTT {
                host.maxRTT = Swift.max(max, rtt)
            } else {
                host.maxRTT = rtt
            }
        }

        let total = host.successCount + host.failureCount
        if total > 0 {
            let pct = Double(host.successCount) / Double(total) * 100.0
            host.successHistory.append(pct)
            if host.successHistory.count > 40 {
                host.successHistory.removeFirst(host.successHistory.count - 40)
            }
            changed = true
        }

        let sample = PingSample(
            timestamp: Date(),
            success: result.success,
            rttMilliseconds: result.rtt.map { $0 * 1000 },
            error: result.errorDescription
        )
        host.lastPingAt = Date()
        host.samples.append(sample)
        if host.samples.count > 100 {
            host.samples.removeFirst(host.samples.count - 100)
        }
        changed = true

        if changed {
            hosts[index] = host
            schedulePersist()
        }
    }

    private func recordHeartbeat(for id: Host.ID) {
        loopHeartbeats[id] = Date()
    }

    private static func normalizeHosts(_ rawInput: String) -> [ParsedHost] {
        let separators = CharacterSet.whitespacesAndNewlines.union(CharacterSet(charactersIn: ",;"))
        let tokens = rawInput
            .components(separatedBy: separators)
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
        return normalizeHostTokens(tokens)
    }

    private static func normalizeHostTokens(_ tokens: [String]) -> [ParsedHost] {
        var deduped: [ParsedHost] = []
        var seen = Set<String>()
        for token in tokens {
            let trimmed = token.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !trimmed.isEmpty else { continue }
            let expanded = expandToken(trimmed)
            for raw in expanded {
                let parsed = parseHostToken(raw)
                let key = canonicalKey(for: parsed)
                if seen.insert(key).inserted {
                    deduped.append(parsed)
                }
            }
        }
        return deduped
    }

    private static func expandToken(_ token: String) -> [String] {
        if let range = ipv4Range(from: token) {
            return range
        }
        if let cidr = cidrRange(from: token) {
            return cidr
        }
        return [token]
    }

    private static func parseHostToken(_ raw: String) -> ParsedHost {
        var backend: PingBackendKind? = nil
        var token = raw

        if token.lowercased().hasPrefix("tcp:") {
            backend = .tcp
            token = String(token.dropFirst(4))
        } else if token.lowercased().hasPrefix("icmp:") {
            backend = .swiftNative
            token = String(token.dropFirst(5))
        }

        let split = splitHostAndPort(token)
        if backend == nil, split.port != nil {
            backend = .tcp
        }

        return ParsedHost(address: split.host, tcpPortOverride: split.port, backend: backend ?? .swiftNative)
    }

    private static func splitHostAndPort(_ raw: String) -> (host: String, port: Int?) {
        if raw.hasPrefix("["), let closing = raw.firstIndex(of: "]") {
            let hostPart = String(raw[raw.index(after: raw.startIndex)..<closing])
            let remainder = raw[raw.index(after: closing)...]
            if remainder.first == ":", let p = Int(remainder.dropFirst()), (1...65535).contains(p) {
                return (hostPart, p)
            }
            return (hostPart, nil)
        }

        if let idx = raw.lastIndex(of: ":"), raw[..<idx].contains(":") == false {
            let hostPart = String(raw[..<idx])
            let portPart = raw[raw.index(after: idx)...]
            if let p = Int(portPart), (1...65535).contains(p) {
                return (hostPart, p)
            }
        }

        return (raw, nil)
    }

    private static func canonicalKey(for host: ParsedHost) -> String {
        canonicalKey(address: host.address, port: host.tcpPortOverride, backend: host.backend ?? .swiftNative)
    }

    private static func canonicalKey(address: String, port: Int?, backend: PingBackendKind) -> String {
        "\(address.lowercased())|\(port.map(String.init) ?? "-")|\(backend.rawValue)"
    }

    private static func ipv4Range(from token: String) -> [String]? {
        guard token.contains("-") else { return nil }
        let parts = token.split(separator: "-")
        guard parts.count == 2 else { return nil }
        let start = String(parts[0])
        let endPart = String(parts[1])
        guard let startIP = IPv4Address(start) else { return nil }
        let endIPString: String
        if endPart.contains(".") {
            endIPString = endPart
        } else {
            // shorthand last octet
            let base = start.split(separator: ".").dropLast().joined(separator: ".")
            endIPString = "\(base).\(endPart)"
        }
        guard let endIP = IPv4Address(endIPString) else { return nil }
        let startInt = startIP.toInt()
        let endInt = endIP.toInt()
        guard endInt >= startInt else { return nil }
        let count = min(endInt - startInt + 1, 1024)
        return (0..<count).compactMap { offset in IPv4Address(intValue: startInt + offset)?.description }
    }

    private static func cidrRange(from token: String) -> [String]? {
        let parts = token.split(separator: "/")
        guard parts.count == 2, let ip = IPv4Address(String(parts[0])), let prefix = Int(parts[1]), prefix >= 0, prefix <= 32 else { return nil }
        let baseInt = ip.toInt()
        let hostBits = 32 - prefix
        if hostBits >= 16 { return nil } // avoid huge expansions
        let count = 1 << hostBits
        return (0..<count).compactMap { offset in IPv4Address(intValue: baseInt + offset)?.description }
    }

    private func maybeNotifyTransition(previousSuccess: Bool?, nowSuccess: Bool) {
        guard let previousSuccess else { return }
        let now = Date()
        if !nowSuccess && previousSuccess {
            if shouldNotify(kind: .failure, now: now) {
                if settings.notificationsEnabled {
                    Task { await soundQueue.playSound(named: settings.failureSoundName) }
                }
                if settings.popupNotificationsEnabled {
                    NotificationHelper.send(title: "Ping failure", body: "First failure after success")
                }
                lastFailureNotification = now
            }
        } else if nowSuccess && !previousSuccess {
            if shouldNotify(kind: .success, now: now) {
                if settings.notificationsEnabled {
                    Task { await soundQueue.playSound(named: settings.clearSoundName) }
                }
                if settings.popupNotificationsEnabled {
                    NotificationHelper.send(title: "Ping recovered", body: "First success after failure")
                }
                lastSuccessNotification = now
            }
        }
    }

    private enum NotificationKind {
        case success
        case failure
    }

    private func shouldNotify(kind: NotificationKind, now: Date) -> Bool {
        let cooldown = settings.notificationCooldownSeconds
        switch kind {
        case .success:
            guard let lastSuccessNotification else { return true }
            return now.timeIntervalSince(lastSuccessNotification) >= cooldown
        case .failure:
            guard let lastFailureNotification else { return true }
            return now.timeIntervalSince(lastFailureNotification) >= cooldown
        }
    }

    private func activeBackend() -> PingBackend {
        switch settings.backend {
        case .swiftNative:
            return swiftBackend
        case .tcp:
            return tcpBackend
        }
    }

    private func reverseLookup(ip: String) async -> String? {
        if let cached = reverseCache[ip] {
            return cached
        }
        if reverseInFlight.contains(ip) {
            return nil
        }
        reverseInFlight.insert(ip)
        defer { reverseInFlight.remove(ip) }

        return await withCheckedContinuation { (continuation: CheckedContinuation<String?, Never>) in
            DispatchQueue.global(qos: .utility).async {
                var addr = sockaddr_storage()
                var len: socklen_t = 0

                if inet_pton(AF_INET, ip, &addr) == 1 {
                    addr.ss_family = sa_family_t(AF_INET)
                    len = socklen_t(MemoryLayout<sockaddr_in>.size)
                } else if inet_pton(AF_INET6, ip, &addr) == 1 {
                    addr.ss_family = sa_family_t(AF_INET6)
                    len = socklen_t(MemoryLayout<sockaddr_in6>.size)
                } else {
                    continuation.resume(returning: nil)
                    return
                }

                var hostBuffer = [CChar](repeating: 0, count: Int(NI_MAXHOST))
                let result = withUnsafePointer(to: &addr) {
                    $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { ptr in
                        getnameinfo(ptr, len, &hostBuffer, socklen_t(hostBuffer.count), nil, 0, NI_NAMEREQD)
                    }
                }

                if result == 0 {
                    let name = String(cString: hostBuffer)
                    continuation.resume(returning: name)
                } else {
                    continuation.resume(returning: nil)
                }
            }
        }.map { name in
            reverseCache[ip] = name
            return name
        }
    }

    private func schedulePersist() {
        persistTask?.cancel()
        persistTask = Task { [weak self] in
            try? await Task.sleep(nanoseconds: 300_000_000)
            self?.persistState()
        }
    }

    private func persistState() {
        let snapshot = PersistedState(
            hosts: hosts.map(\.addressWithPort),
            hostsV2: hosts.map { StoredHost(address: $0.address, tcpPortOverride: $0.tcpPortOverride, backend: $0.backend.rawValue) },
            intervalSeconds: settings.intervalSeconds,
            timeoutSeconds: settings.timeoutSeconds,
            notificationsEnabled: settings.notificationsEnabled,
            popupNotificationsEnabled: settings.popupNotificationsEnabled,
            notificationCooldownSeconds: settings.notificationCooldownSeconds,
            toolbarIconStyle: settings.toolbarIconStyle.rawValue,
            failureSoundName: settings.failureSoundName,
            clearSoundName: settings.clearSoundName,
            backend: settings.backend.rawValue,
            maxConcurrentPings: settings.maxConcurrentPings,
            pingPayloadBytes: settings.pingPayloadBytes,
            ttl: settings.ttl,
            doNotFragment: settings.doNotFragment,
            defaultTCPPort: settings.defaultTCPPort,
            visibleColumns: Array(visibleColumns.map(\.rawValue))
        )
        let url = persistenceURL
        Task.detached {
            do {
                let data = try JSONEncoder().encode(snapshot)
                let dir = url.deletingLastPathComponent()
                try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
                try data.write(to: url, options: .atomic)
            } catch {
                // Best-effort persistence; ignore errors.
            }
        }
    }

    private func startRelativeTimer() {
        relativeTimer?.invalidate()
        relativeTimer = Timer.scheduledTimer(withTimeInterval: 5.0, repeats: true) { [weak self] _ in
            guard let self else { return }
            Task { @MainActor in
                self.relativeNow = Date()
                self.checkLoopHealth()
            }
        }
    }

    private func checkLoopHealth() {
        let now = Date()
        for host in hosts where !host.paused {
            let beat = loopHeartbeats[host.id]
            let interval = max(host.interval, 0.5)
            let threshold = max(5.0, interval * 3.0)
            if let beat, now.timeIntervalSince(beat) < threshold {
                continue
            }
            startLoop(for: host.id)
        }
    }

    private func loadPersisted() {
        guard FileManager.default.fileExists(atPath: persistenceURL.path) else { return }
        guard let data = try? Data(contentsOf: persistenceURL),
              let state = try? JSONDecoder().decode(PersistedState.self, from: data) else { return }

        settings.intervalSeconds = state.intervalSeconds
        settings.timeoutSeconds = state.timeoutSeconds
        settings.notificationsEnabled = state.notificationsEnabled ?? false
        settings.popupNotificationsEnabled = state.popupNotificationsEnabled ?? false
        settings.notificationCooldownSeconds = state.notificationCooldownSeconds ?? MonitoringSettings.default.notificationCooldownSeconds
        if let styleRaw = state.toolbarIconStyle, let style = ToolbarIconStyle(rawValue: styleRaw) {
            settings.toolbarIconStyle = style
        }
        if let backendRaw = state.backend, let backend = PingBackendKind(rawValue: backendRaw) {
            settings.backend = backend
        }
        if let maxConcurrent = state.maxConcurrentPings {
            settings.maxConcurrentPings = maxConcurrent
        }
        if let size = state.pingPayloadBytes {
            settings.pingPayloadBytes = size
        }
        if let ttl = state.ttl {
            settings.ttl = ttl
        }
        if let df = state.doNotFragment {
            settings.doNotFragment = df
        }
        if let tcpPort = state.defaultTCPPort {
            settings.defaultTCPPort = tcpPort
        }
        if let failure = state.failureSoundName {
            settings.failureSoundName = failure
        }
        if let clear = state.clearSoundName {
            settings.clearSoundName = clear
        }
        if let columns = state.visibleColumns {
            let keys = columns.compactMap { ColumnKey(rawValue: $0) }
            if !keys.isEmpty {
                visibleColumns = Set(keys)
            }
        }

        let parsedHosts: [ParsedHost]
        if let stored = state.hostsV2 {
            parsedHosts = stored.map { ParsedHost(address: $0.address, tcpPortOverride: $0.tcpPortOverride, backend: $0.backend.flatMap(PingBackendKind.init(rawValue:)) ?? .swiftNative) }
        } else {
            parsedHosts = Self.normalizeHostTokens(state.hosts)
        }
        for host in parsedHosts {
            let added = Host(
                address: host.address,
                resolvedIP: nil,
                resolvedName: nil,
                lastRTT: nil,
                lastError: nil,
                lastOK: nil,
                successCount: 0,
                failureCount: 0,
                lastSuccess: nil,
                rttHistory: [],
                successHistory: [],
                samples: [],
                interval: settings.intervalSeconds,
                timeout: settings.timeoutSeconds,
                paused: false,
                backend: host.backend ?? .swiftNative,
                tcpPortOverride: host.tcpPortOverride,
                minRTT: nil,
                maxRTT: nil,
                totalRTT: 0,
                rttSampleCount: 0
            )
            hosts.append(added)
            startLoop(for: added.id)
        }
    }
}

struct AddHostResult {
    let added: [Host]
    let duplicates: [String]
}

private struct StoredHost: Codable {
    let address: String
    let tcpPortOverride: Int?
    let backend: String?
}

private struct ParsedHost: Hashable {
    let address: String
    let tcpPortOverride: Int?
    let backend: PingBackendKind?

    var displayString: String {
        switch backend {
        case .tcp:
            let suffix: String
            if let tcpPortOverride {
                if address.contains(":") && !address.hasPrefix("[") {
                    suffix = "[\(address)]:\(tcpPortOverride)"
                } else {
                    suffix = "\(address):\(tcpPortOverride)"
                }
            } else {
                suffix = address
            }
            return "tcp:\(suffix)"
        default:
            guard let tcpPortOverride else { return address }
            if address.contains(":") && !address.hasPrefix("[") {
                return "[\(address)]:\(tcpPortOverride)"
            }
            return "\(address):\(tcpPortOverride)"
        }
    }

    func hash(into hasher: inout Hasher) {
        hasher.combine(address.lowercased())
        hasher.combine(tcpPortOverride ?? 0)
        hasher.combine(backend?.rawValue ?? "icmp")
    }

    static func == (lhs: ParsedHost, rhs: ParsedHost) -> Bool {
        lhs.address.caseInsensitiveCompare(rhs.address) == .orderedSame && lhs.tcpPortOverride == rhs.tcpPortOverride && lhs.backend == rhs.backend
    }
}

private struct PersistedState: Codable {
    let hosts: [String]
    let hostsV2: [StoredHost]?
    let intervalSeconds: Double
    let timeoutSeconds: Double
    let notificationsEnabled: Bool?
    let popupNotificationsEnabled: Bool?
    let notificationCooldownSeconds: Double?
    let toolbarIconStyle: String?
    let failureSoundName: String?
    let clearSoundName: String?
    let backend: String?
    let maxConcurrentPings: Int?
    let pingPayloadBytes: Int?
    let ttl: Int?
    let doNotFragment: Bool?
    let defaultTCPPort: Int?
    let visibleColumns: [String]?
}

private struct IPv4Address {
    let octets: (UInt8, UInt8, UInt8, UInt8)

    init?(_ string: String) {
        let parts = string.split(separator: ".")
        guard parts.count == 4,
              let a = UInt8(parts[0]),
              let b = UInt8(parts[1]),
              let c = UInt8(parts[2]),
              let d = UInt8(parts[3]) else { return nil }
        self.octets = (a, b, c, d)
    }

    init?(intValue: Int) {
        guard intValue >= 0 && intValue <= 0xFFFFFFFF else { return nil }
        let a = UInt8((intValue >> 24) & 0xFF)
        let b = UInt8((intValue >> 16) & 0xFF)
        let c = UInt8((intValue >> 8) & 0xFF)
        let d = UInt8(intValue & 0xFF)
        self.octets = (a, b, c, d)
    }

    func toInt() -> Int {
        let (a, b, c, d) = octets
        return Int(a) << 24 | Int(b) << 16 | Int(c) << 8 | Int(d)
    }

    var description: String {
        let (a, b, c, d) = octets
        return "\(a).\(b).\(c).\(d)"
    }
}

final class SoundQueue {
    private var lastTask: Task<Void, Never>?

    @MainActor
    func playSound(named name: String) async {
        let previous = lastTask
        let task = Task { @MainActor in
            await previous?.value
            let sound = NSSound(named: NSSound.Name(name))
            sound?.play()
            let duration = sound?.duration ?? 0.5
            try? await Task.sleep(nanoseconds: UInt64(duration * 1_000_000_000))
        }
        lastTask = task
        await task.value
    }
}
