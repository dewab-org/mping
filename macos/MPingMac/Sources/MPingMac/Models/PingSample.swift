import Foundation

struct PingSample: Identifiable, Codable, Hashable {
    let id: UUID
    let timestamp: Date
    let success: Bool
    let rttMilliseconds: Double?
    let error: String?

    init(id: UUID = UUID(), timestamp: Date, success: Bool, rttMilliseconds: Double?, error: String?) {
        self.id = id
        self.timestamp = timestamp
        self.success = success
        self.rttMilliseconds = rttMilliseconds
        self.error = error
    }
}
