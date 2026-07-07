import Foundation

struct PingResult: Sendable {
    let host: String
    let success: Bool
    let rtt: TimeInterval?
    let resolvedIP: String?
    let resolvedName: String?
    let rawOutput: String
    let errorDescription: String?
}
