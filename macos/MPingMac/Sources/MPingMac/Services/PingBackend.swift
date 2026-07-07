import Foundation
import Network

protocol PingBackend: Sendable {
    func ping(host: String, timeout: TimeInterval, payloadSize: Int, ttl: Int, doNotFragment: Bool, tcpPort: Int) async -> PingResult
}

// Swift-native ICMP (IPv4 & IPv6) echo using raw sockets.
struct SwiftICMPBackend: PingBackend {
    func ping(host: String, timeout: TimeInterval, payloadSize: Int, ttl: Int, doNotFragment: Bool, tcpPort: Int) async -> PingResult {
        let resolved = await resolveHost(host)
        guard let target = resolved else {
            return PingResult(host: host, success: false, rtt: nil, resolvedIP: nil, resolvedName: nil, rawOutput: "", errorDescription: "Could not resolve host")
        }

        switch target {
        case .ipv4(let addr):
            return await pingIPv4(address: addr, host: host, timeout: timeout, payloadSize: payloadSize, ttl: ttl, doNotFragment: doNotFragment)
        case .ipv6(let addr):
            return await pingIPv6(address: addr, host: host, timeout: timeout, payloadSize: payloadSize, ttl: ttl)
        }
    }

    private enum Target {
        case ipv4(sockaddr_in)
        case ipv6(sockaddr_in6)
    }

    private func resolveHost(_ host: String) async -> Target? {
        await resolveHost(host, attempts: 3)
    }

    private func resolveHost(_ host: String, attempts: Int) async -> Target? {
        let clampedAttempts = max(1, attempts)
        for attempt in 0..<clampedAttempts {
            if let target = resolveHostOnce(host) { return target }
            if attempt < clampedAttempts - 1 {
                try? await Task.sleep(nanoseconds: UInt64(50_000_000 * UInt64(attempt + 1)))
            }
        }
        return nil
    }

    private func resolveHostOnce(_ host: String) -> Target? {
        var hints = addrinfo(
            ai_flags: AI_DEFAULT,
            ai_family: AF_UNSPEC,
            ai_socktype: SOCK_DGRAM,
            ai_protocol: 0,
            ai_addrlen: 0,
            ai_canonname: nil,
            ai_addr: nil,
            ai_next: nil
        )
        var infoPtr: UnsafeMutablePointer<addrinfo>?
        defer { freeaddrinfo(infoPtr) }

        if getaddrinfo(host, nil, &hints, &infoPtr) == 0, let first = infoPtr {
            var cursor: UnsafeMutablePointer<addrinfo>? = first
            while let current = cursor {
                if current.pointee.ai_family == AF_INET, let addr = current.pointee.ai_addr?.withMemoryRebound(to: sockaddr_in.self, capacity: 1, { $0.pointee }) {
                    return .ipv4(addr)
                } else if current.pointee.ai_family == AF_INET6, let addr6 = current.pointee.ai_addr?.withMemoryRebound(to: sockaddr_in6.self, capacity: 1, { $0.pointee }) {
                    return .ipv6(addr6)
                }
                cursor = current.pointee.ai_next
            }
        }

        // Fallback to legacy resolver for cases where getaddrinfo fails but mDNS/DNS works (e.g., short hostnames).
        if let fallback4: sockaddr_in = resolveWithGethostbyname4(host, family: AF_INET) {
            return .ipv4(fallback4)
        }
        if let fallback6: sockaddr_in6 = resolveWithGethostbyname6(host, family: AF_INET6) {
            return .ipv6(fallback6)
        }
        return nil
    }

    private func pingIPv4(address: sockaddr_in, host: String, timeout: TimeInterval, payloadSize: Int, ttl: Int, doNotFragment: Bool) async -> PingResult {
        let sock = socket(AF_INET, SOCK_DGRAM, IPPROTO_ICMP)
        if sock < 0 {
            return PingResult(host: host, success: false, rtt: nil, resolvedIP: host, resolvedName: nil, rawOutput: "", errorDescription: "Socket error")
        }
        defer { close(sock) }

        var ttlVal = Int32(ttl)
        setsockopt(sock, IPPROTO_IP, IP_TTL, &ttlVal, socklen_t(MemoryLayout<Int32>.size))
        if doNotFragment {
            var dontFrag: Int32 = 1
            setsockopt(sock, IPPROTO_IP, IP_DONTFRAG, &dontFrag, socklen_t(MemoryLayout<Int32>.size))
        }

        let micro = Int32((timeout.truncatingRemainder(dividingBy: 1)) * 1_000_000)
        var tv = timeval(tv_sec: Int(timeout), tv_usec: micro)
        setsockopt(sock, SOL_SOCKET, SO_RCVTIMEO, &tv, socklen_t(MemoryLayout<timeval>.size))

        let icmp = ICMPHeader(type: 8, code: 0, checksum: 0, identifier: UInt16(getpid() & 0xffff), sequenceNumber: 0)
        var packet = icmp.data
        let dataSize = max(0, min(payloadSize, 2048))
        packet.append(contentsOf: Array(repeating: 0, count: dataSize))
        let checksum = Self.checksum(data: packet)
        packet[2] = UInt8(checksum >> 8)
        packet[3] = UInt8(checksum & 0xff)

        let start = Date()
        let sent = packet.withUnsafeBytes { ptr -> ssize_t in
            var dest = address
            return withUnsafePointer(to: &dest) {
                $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { saPtr in
                    sendto(sock, ptr.baseAddress, ptr.count, 0, saPtr, socklen_t(MemoryLayout<sockaddr_in>.size))
                }
            }
        }
        guard sent == packet.count else {
            return PingResult(host: host, success: false, rtt: nil, resolvedIP: host, resolvedName: nil, rawOutput: "", errorDescription: "Send failed")
        }

        var buffer = [UInt8](repeating: 0, count: 512)
        let count = buffer.count
        var replyAddressStorage = sockaddr_in()
        let targetIP = string(from: address) ?? host
        let matchedReply: String? = {
            while true {
                let received = buffer.withUnsafeMutableBytes { bufPtr -> ssize_t in
                    var len: socklen_t = socklen_t(MemoryLayout<sockaddr_in>.size)
                    return withUnsafeMutablePointer(to: &replyAddressStorage) {
                        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { saPtr in
                            recvfrom(sock, bufPtr.baseAddress, count, 0, saPtr, &len)
                        }
                    }
                }
                if received <= 0 {
                    return nil
                }

                let replyAddress = string(from: replyAddressStorage) ?? ""
                if replyAddress != targetIP {
                    continue
                }

                let icmpStart: Int
                if received >= 8, buffer[0] == 0 {
                    icmpStart = 0 // Datagram sockets on macOS typically omit the IP header.
                } else if received >= 28, buffer[20] == 0 {
                    icmpStart = 20 // Fallback if an IP header is present.
                } else {
                    return nil
                }

                let type = buffer[icmpStart]
                if type != 0 || received < icmpStart + 8 {
                    continue
                }

                let id = UInt16(buffer[icmpStart + 4]) << 8 | UInt16(buffer[icmpStart + 5])
                if id != icmp.identifier {
                    continue
                }

                return replyAddress
            }
        }()

        guard matchedReply != nil else {
            return PingResult(host: host, success: false, rtt: nil, resolvedIP: host, resolvedName: nil, rawOutput: "", errorDescription: "Timeout")
        }

        let rtt = Date().timeIntervalSince(start)
        let resolved = string(from: address)
        let reverseName = hostname(from: replyAddressStorage)
        return PingResult(host: host, success: true, rtt: rtt, resolvedIP: resolved ?? host, resolvedName: reverseName, rawOutput: "ICMPv4 reply", errorDescription: nil)
    }

    private func pingIPv6(address: sockaddr_in6, host: String, timeout: TimeInterval, payloadSize: Int, ttl: Int) async -> PingResult {
        let sock = socket(AF_INET6, SOCK_DGRAM, IPPROTO_ICMPV6)
        if sock < 0 {
            return PingResult(host: host, success: false, rtt: nil, resolvedIP: host, resolvedName: nil, rawOutput: "", errorDescription: "Socket error")
        }
        defer { close(sock) }

        var hops = Int32(ttl)
        setsockopt(sock, IPPROTO_IPV6, IPV6_UNICAST_HOPS, &hops, socklen_t(MemoryLayout<Int32>.size))

        let micro = Int32((timeout.truncatingRemainder(dividingBy: 1)) * 1_000_000)
        var tv = timeval(tv_sec: Int(timeout), tv_usec: micro)
        setsockopt(sock, SOL_SOCKET, SO_RCVTIMEO, &tv, socklen_t(MemoryLayout<timeval>.size))

        let icmp = ICMPv6Header(type: 128, code: 0, checksum: 0, identifier: UInt16(getpid() & 0xffff), sequenceNumber: 0)
        var packet = icmp.data
        let dataSize = max(0, min(payloadSize, 2048))
        packet.append(contentsOf: Array(repeating: 0, count: dataSize))

        let start = Date()
        let sent = packet.withUnsafeBytes { ptr -> ssize_t in
            var dest = address
            return withUnsafePointer(to: &dest) {
                $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { saPtr in
                    sendto(sock, ptr.baseAddress, ptr.count, 0, saPtr, socklen_t(MemoryLayout<sockaddr_in6>.size))
                }
            }
        }
        guard sent == packet.count else {
            return PingResult(host: host, success: false, rtt: nil, resolvedIP: host, resolvedName: nil, rawOutput: "", errorDescription: "Send failed")
        }

        var buffer = [UInt8](repeating: 0, count: 512)
        let count = buffer.count
        var replyAddressStorage = sockaddr_in6()
        let targetIP = string(from: address) ?? host
        let matchedReply: String? = {
            while true {
                let received = buffer.withUnsafeMutableBytes { bufPtr -> ssize_t in
                    var len: socklen_t = socklen_t(MemoryLayout<sockaddr_in6>.size)
                    return withUnsafeMutablePointer(to: &replyAddressStorage) {
                        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { saPtr in
                            recvfrom(sock, bufPtr.baseAddress, count, 0, saPtr, &len)
                        }
                    }
                }
                if received <= 0 {
                    return nil
                }

                let replyAddress = string(from: replyAddressStorage) ?? ""
                if replyAddress != targetIP {
                    continue
                }

                if received >= 8 {
                    let type = buffer[0]
                    if type == 129 {
                        let id = UInt16(buffer[4]) << 8 | UInt16(buffer[5])
                        if id != icmp.identifier {
                            continue
                        }
                        return replyAddress
                    }
                }
            }
        }()

        guard matchedReply != nil else {
            return PingResult(host: host, success: false, rtt: nil, resolvedIP: host, resolvedName: nil, rawOutput: "", errorDescription: "Timeout")
        }

        let rtt = Date().timeIntervalSince(start)
        let resolved = string(from: address)
        let reverseName = hostname(from: replyAddressStorage)
        return PingResult(host: host, success: true, rtt: rtt, resolvedIP: resolved ?? host, resolvedName: reverseName, rawOutput: "ICMPv6 reply", errorDescription: nil)
    }

    private static func checksum(data: [UInt8]) -> UInt16 {
        var sum: UInt32 = 0
        var bytes = data
        if bytes.count % 2 != 0 {
            bytes.append(0)
        }
        for i in stride(from: 0, to: bytes.count, by: 2) {
            let word = UInt16(bytes[i]) << 8 | UInt16(bytes[i + 1])
            sum &+= UInt32(word)
        }
        while (sum >> 16) != 0 {
            sum = (sum & 0xFFFF) + (sum >> 16)
        }
        return ~UInt16(sum & 0xFFFF)
    }
}

struct TCPPingBackend: PingBackend {
    func ping(host: String, timeout: TimeInterval, payloadSize: Int, ttl: Int, doNotFragment: Bool, tcpPort: Int) async -> PingResult {
        let parsed = parseHostAndPort(host, defaultPort: tcpPort)
        let resolvedIP = resolveIPAddress(parsed.host)
        let resolvedName = resolvedIP.flatMap(resolveHostname) ?? host
        let maxPayload = 1460 // approximate safe payload for MTU 1500
        if payloadSize > maxPayload {
            return PingResult(
                host: host,
                success: false,
                rtt: nil,
                resolvedIP: resolvedIP,
                resolvedName: resolvedName,
                rawOutput: "TCP payload too large (\(payloadSize) > \(maxPayload))",
                errorDescription: "Payload exceeds MTU for TCP"
            )
        }
        let parameters = NWParameters.tcp
        let start = Date()

        return await withCheckedContinuation { continuation in
            let connection = NWConnection(host: NWEndpoint.Host(parsed.host), port: parsed.port, using: parameters)
            let flag = ResumeOnceFlag()

            let finish: @Sendable (Bool, String?) -> Void = { success, error in
                guard flag.markResumed() else { return }
                connection.cancel()
                let rtt = Date().timeIntervalSince(start)
                let remoteIP = connection.currentPath?.remoteEndpoint?.ipString
                let reverseName = remoteIP.flatMap(resolveHostname)
                let nameToUse = reverseName ?? resolvedName
                let friendlyError = shortError(error)
                let result = PingResult(
                    host: host,
                    success: success,
                    rtt: success ? rtt : nil,
                    resolvedIP: resolvedIP ?? remoteIP,
                    resolvedName: nameToUse,
                    rawOutput: success ? "TCP connect succeeded" : "TCP connect failed: \(error ?? "Unknown error")",
                    errorDescription: success ? nil : (friendlyError ?? "TCP connect failed")
                )
                continuation.resume(returning: result)
            }

            connection.stateUpdateHandler = { state in
                switch state {
                case .ready:
                    finish(true, nil)
                case .failed(let err):
                    finish(false, err.localizedDescription)
                case .waiting(let err):
                    finish(false, err.localizedDescription)
                default:
                    break
                }
            }

            connection.start(queue: .global())

            DispatchQueue.global().asyncAfter(deadline: .now() + timeout) {
                finish(false, "TCP timeout")
            }
        }
    }
}

private struct ICMPHeader {
    var type: UInt8
    var code: UInt8
    var checksum: UInt16
    var identifier: UInt16
    var sequenceNumber: UInt16

    var data: [UInt8] {
        let bytes: [UInt8] = [
            type,
            code,
            0, 0,
            UInt8(identifier >> 8), UInt8(identifier & 0xff),
            UInt8(sequenceNumber >> 8), UInt8(sequenceNumber & 0xff)
        ]
        return bytes
    }
}

private struct ICMPv6Header {
    var type: UInt8
    var code: UInt8
    var checksum: UInt16
    var identifier: UInt16
    var sequenceNumber: UInt16

    var data: [UInt8] {
        let bytes: [UInt8] = [
            type,
            code,
            0, 0,
            UInt8(identifier >> 8), UInt8(identifier & 0xff),
            UInt8(sequenceNumber >> 8), UInt8(sequenceNumber & 0xff)
        ]
        return bytes
    }
}

private extension NWEndpoint {
    var ipString: String? {
        switch self {
        case .hostPort(let host, _):
            switch host {
            case .ipv4(let addr):
                return addr.debugDescription
            case .ipv6(let addr):
                return addr.debugDescription
            case .name:
                return nil
            @unknown default:
                return host.debugDescription
            }
        default:
            return nil
        }
    }
}

private func resolveIPAddress(_ host: String) -> String? {
    var hints = addrinfo(
        ai_flags: AI_DEFAULT,
        ai_family: AF_UNSPEC,
        ai_socktype: SOCK_STREAM,
        ai_protocol: 0,
        ai_addrlen: 0,
        ai_canonname: nil,
        ai_addr: nil,
        ai_next: nil
    )
    var infoPtr: UnsafeMutablePointer<addrinfo>?
    defer { freeaddrinfo(infoPtr) }

    if getaddrinfo(host, nil, &hints, &infoPtr) == 0, let first = infoPtr {
        var cursor: UnsafeMutablePointer<addrinfo>? = first
        while let current = cursor {
            if current.pointee.ai_family == AF_INET, let addr = current.pointee.ai_addr?.withMemoryRebound(to: sockaddr_in.self, capacity: 1, { $0.pointee }) {
                return string(from: addr)
            } else if current.pointee.ai_family == AF_INET6, let addr6 = current.pointee.ai_addr?.withMemoryRebound(to: sockaddr_in6.self, capacity: 1, { $0.pointee }) {
                return string(from: addr6)
            }
            cursor = current.pointee.ai_next
        }
    }

    if let fallback4: sockaddr_in = resolveWithGethostbyname4(host, family: AF_INET) {
        return string(from: fallback4)
    }
    if let fallback6: sockaddr_in6 = resolveWithGethostbyname6(host, family: AF_INET6) {
        return string(from: fallback6)
    }
    return nil
}

private func resolveHostname(_ ip: String) -> String? {
    var addr4 = sockaddr_in()
    var addr6 = sockaddr_in6()
    if ip.withCString({ inet_pton(AF_INET, $0, &addr4.sin_addr) }) == 1 {
        addr4.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
        addr4.sin_family = sa_family_t(AF_INET)
        return hostname(from: addr4)
    } else if ip.withCString({ inet_pton(AF_INET6, $0, &addr6.sin6_addr) }) == 1 {
        addr6.sin6_len = UInt8(MemoryLayout<sockaddr_in6>.size)
        addr6.sin6_family = sa_family_t(AF_INET6)
        return hostname(from: addr6)
    }
    return nil
}

private func string(from address: sockaddr_in) -> String? {
    var addr = address
    var buffer = [CChar](repeating: 0, count: Int(NI_MAXHOST))
    return withUnsafePointer(to: &addr) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { saPtr in
            let res = getnameinfo(saPtr, socklen_t(MemoryLayout<sockaddr_in>.size), &buffer, socklen_t(buffer.count), nil, 0, NI_NUMERICHOST)
            return res == 0 ? String(cString: buffer) : nil
        }
    }
}

private func hostname(from address: sockaddr_in) -> String? {
    var addr = address
    var buffer = [CChar](repeating: 0, count: Int(NI_MAXHOST))
    return withUnsafePointer(to: &addr) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { saPtr in
            let res = getnameinfo(saPtr, socklen_t(MemoryLayout<sockaddr_in>.size), &buffer, socklen_t(buffer.count), nil, 0, NI_NAMEREQD)
            return res == 0 ? String(cString: buffer) : nil
        }
    }
}

private func string(from address: sockaddr_in6) -> String? {
    var addr = address
    var buffer = [CChar](repeating: 0, count: Int(NI_MAXHOST))
    return withUnsafePointer(to: &addr) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { saPtr in
            let res = getnameinfo(saPtr, socklen_t(MemoryLayout<sockaddr_in6>.size), &buffer, socklen_t(buffer.count), nil, 0, NI_NUMERICHOST)
            return res == 0 ? String(cString: buffer) : nil
        }
    }
}

private func hostname(from address: sockaddr_in6) -> String? {
    var addr = address
    var buffer = [CChar](repeating: 0, count: Int(NI_MAXHOST))
    return withUnsafePointer(to: &addr) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { saPtr in
            let res = getnameinfo(saPtr, socklen_t(MemoryLayout<sockaddr_in6>.size), &buffer, socklen_t(buffer.count), nil, 0, NI_NAMEREQD)
            return res == 0 ? String(cString: buffer) : nil
        }
    }
}

private func resolveWithGethostbyname4(_ host: String, family: Int32) -> sockaddr_in? {
    guard let ent = gethostbyname2(host, family) else { return nil }
    guard ent.pointee.h_length >= MemoryLayout<in_addr>.size else { return nil }
    if family == AF_INET, let addrList = ent.pointee.h_addr_list, let first = addrList.pointee {
        let addrIn = UnsafeRawPointer(first).assumingMemoryBound(to: in_addr.self).pointee
        var sockaddr = sockaddr_in()
        sockaddr.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
        sockaddr.sin_family = sa_family_t(AF_INET)
        sockaddr.sin_addr = addrIn
        return sockaddr
    }
    return nil
}

private func resolveWithGethostbyname6(_ host: String, family: Int32) -> sockaddr_in6? {
    guard let ent = gethostbyname2(host, family) else { return nil }
    guard ent.pointee.h_length >= MemoryLayout<in6_addr>.size else { return nil }
    if family == AF_INET6, let addrList = ent.pointee.h_addr_list, let first = addrList.pointee {
        let addrIn6 = UnsafeRawPointer(first).assumingMemoryBound(to: in6_addr.self).pointee
        var sockaddr = sockaddr_in6()
        sockaddr.sin6_len = UInt8(MemoryLayout<sockaddr_in6>.size)
        sockaddr.sin6_family = sa_family_t(AF_INET6)
        sockaddr.sin6_addr = addrIn6
        return sockaddr
    }
    return nil
}

private func parseHostAndPort(_ raw: String, defaultPort: Int) -> (host: String, port: NWEndpoint.Port) {
    let clampedDefault = max(1, min(defaultPort, 65535))
    if raw.hasPrefix("["), let closing = raw.firstIndex(of: "]") {
        let hostPart = String(raw[raw.index(after: raw.startIndex)..<closing])
        let remainder = raw[raw.index(after: closing)...]
        if remainder.first == ":", let p = Int(remainder.dropFirst()), (1...65535).contains(p), let port = NWEndpoint.Port(rawValue: UInt16(p)) {
            return (hostPart, port)
        }
        return (hostPart, NWEndpoint.Port(rawValue: UInt16(clampedDefault)) ?? .http)
    }

    if let idx = raw.lastIndex(of: ":"), raw[..<idx].contains(":") == false {
        let hostPart = String(raw[..<idx])
        let portPart = raw[raw.index(after: idx)...]
        if let p = Int(portPart), (1...65535).contains(p), let port = NWEndpoint.Port(rawValue: UInt16(p)) {
            return (hostPart, port)
        }
    }

    return (raw, NWEndpoint.Port(rawValue: UInt16(clampedDefault)) ?? .http)
}

final class ResumeOnceFlag: @unchecked Sendable {
    private let lock = NSLock()
    private var resumed = false

    func markResumed() -> Bool {
        lock.lock()
        defer { lock.unlock() }
        guard !resumed else { return false }
        resumed = true
        return true
    }
}

private func shortError(_ message: String?) -> String? {
    guard let message else { return nil }
    if let range = message.range(of: " - ", options: .backwards) {
        let trimmed = message[range.upperBound...].trimmingCharacters(in: .whitespacesAndNewlines)
        if !trimmed.isEmpty {
            let cleaned = trimmed.trimmingCharacters(in: CharacterSet(charactersIn: "() ").union(.whitespacesAndNewlines))
            if !cleaned.isEmpty { return String(cleaned) }
        }
    }
    return message.trimmingCharacters(in: CharacterSet(charactersIn: "() ").union(.whitespacesAndNewlines))
}
