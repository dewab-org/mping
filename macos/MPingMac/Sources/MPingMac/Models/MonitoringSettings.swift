import Foundation
import SwiftUI

struct MonitoringSettings: Equatable {
    var backend: PingBackendKind
    var intervalSeconds: Double
    var timeoutSeconds: Double
    var maxConcurrentPings: Int
    var notificationsEnabled: Bool
    var popupNotificationsEnabled: Bool
    var notificationCooldownSeconds: Double
    var toolbarIconStyle: ToolbarIconStyle
    var failureSoundName: String
    var clearSoundName: String
    var pingPayloadBytes: Int
    var ttl: Int
    var doNotFragment: Bool
    var defaultTCPPort: Int

    static let `default` = MonitoringSettings(
        backend: .swiftNative,
        intervalSeconds: 2,
        timeoutSeconds: 2,
        maxConcurrentPings: 64,
        notificationsEnabled: false,
        popupNotificationsEnabled: false,
        notificationCooldownSeconds: 30,
        toolbarIconStyle: .standard,
        failureSoundName: "Basso",
        clearSoundName: "Glass",
        pingPayloadBytes: 32,
        ttl: 64,
        doNotFragment: true,
        defaultTCPPort: 22
    )
}

enum PingBackendKind: String, CaseIterable, Identifiable {
    case swiftNative
    case tcp

    var id: String { rawValue }

    var label: String {
        switch self {
        case .swiftNative:
            return "ICMP Ping"
        case .tcp:
            return "TCP connect"
        }
    }
}

enum ToolbarIconStyle: String, CaseIterable, Identifiable, Codable {
    case standard
    case filled

    var id: String { rawValue }

    var label: String {
        switch self {
        case .standard: return "Standard"
        case .filled: return "Filled"
        }
    }

    var symbolVariant: SymbolVariants? {
        switch self {
        case .standard:
            return nil
        case .filled:
            return .fill
        }
    }
}
