import Foundation

struct Host: Identifiable, Hashable {
    let id: UUID
    var address: String
    var resolvedIP: String?
    var resolvedName: String?
    var lastRTT: TimeInterval?
    var lastError: String?
    var lastOK: Date?
    var successCount: Int
    var failureCount: Int
    var lastSuccess: Bool?
    var rttHistory: [Double]
    var successHistory: [Double]
    var samples: [PingSample]
    var interval: TimeInterval
    var timeout: TimeInterval
    var paused: Bool
    var backend: PingBackendKind
    var tcpPortOverride: Int?
    var lastPingAt: Date?
    var minRTT: TimeInterval?
    var maxRTT: TimeInterval?
    var totalRTT: TimeInterval
    var rttSampleCount: Int

    init(id: UUID = UUID(),
         address: String,
         resolvedIP: String? = nil,
         resolvedName: String? = nil,
         lastRTT: TimeInterval? = nil,
         lastError: String? = nil,
         lastOK: Date? = nil,
         successCount: Int = 0,
         failureCount: Int = 0,
         lastSuccess: Bool? = nil,
         rttHistory: [Double] = [],
         successHistory: [Double] = [],
         samples: [PingSample] = [],
         interval: TimeInterval,
         timeout: TimeInterval,
         paused: Bool = false,
         backend: PingBackendKind = .swiftNative,
         tcpPortOverride: Int? = nil,
         lastPingAt: Date? = nil,
         minRTT: TimeInterval? = nil,
         maxRTT: TimeInterval? = nil,
         totalRTT: TimeInterval = 0,
         rttSampleCount: Int = 0) {
        self.id = id
        self.address = address
        self.resolvedIP = resolvedIP
        self.resolvedName = resolvedName
        self.lastRTT = lastRTT
        self.lastError = lastError
        self.lastOK = lastOK
        self.successCount = successCount
        self.failureCount = failureCount
        self.lastSuccess = lastSuccess
        self.rttHistory = rttHistory
        self.successHistory = successHistory
        self.samples = samples
        self.interval = interval
        self.timeout = timeout
        self.paused = paused
        self.backend = backend
        self.tcpPortOverride = tcpPortOverride
        self.lastPingAt = lastPingAt
        self.minRTT = minRTT
        self.maxRTT = maxRTT
        self.totalRTT = totalRTT
        self.rttSampleCount = rttSampleCount
    }

    var displayName: String {
        if let resolvedName, !resolvedName.isEmpty {
            return resolvedName
        }
        return address
    }

    var successRate: Double {
        let total = successCount + failureCount
        guard total > 0 else { return 0 }
        return Double(successCount) / Double(total)
    }

    var successRatePercent: Double {
        successRate * 100
    }

    var statusSortValue: Int {
        if lastSuccess == true { return 2 }
        if lastSuccess == false { return 1 }
        return 0
    }

    var rttSortValue: Double {
        lastRTT ?? .infinity
    }

    var lastOKSortValue: Date {
        lastOK ?? .distantPast
    }

    var ipSortValue: String {
        resolvedIP ?? ""
    }

    var errorSortValue: String {
        lastError ?? ""
    }

    var minRTTSortValue: Double {
        minRTT ?? .infinity
    }

    var avgRTTSortValue: Double {
        averageRTT ?? .infinity
    }

    var maxRTTSortValue: Double {
        maxRTT ?? .infinity
    }

    var lossSortValue: Double {
        lossRate
    }

    var averageRTT: TimeInterval? {
        guard rttSampleCount > 0 else { return nil }
        return totalRTT / Double(rttSampleCount)
    }

    var lossRate: Double {
        let total = successCount + failureCount
        guard total > 0 else { return 0 }
        return Double(failureCount) / Double(total)
    }

    var addressWithPort: String {
        guard let tcpPortOverride else { return address }
        let needsBrackets = address.contains(":") && !address.hasPrefix("[")
        if needsBrackets {
            return "[\(address)]:\(tcpPortOverride)"
        }
        return "\(address):\(tcpPortOverride)"
    }

    func matches(address other: String, tcpPort: Int?, backend otherBackend: PingBackendKind? = nil) -> Bool {
        let backendMatches = otherBackend == nil || backend == otherBackend
        return backendMatches && tcpPort == tcpPortOverride && address.caseInsensitiveCompare(other) == .orderedSame
    }

    var inputRepresentation: String {
        switch backend {
        case .tcp:
            if tcpPortOverride != nil {
                return "tcp:\(addressWithPort)"
            } else {
                return "tcp:\(address)"
            }
        case .swiftNative:
            return addressWithPort
        }
    }
}
