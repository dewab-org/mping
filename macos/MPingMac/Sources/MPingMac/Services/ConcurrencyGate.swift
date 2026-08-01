import Foundation

actor ConcurrencyGate {
    private var available: Int
    private var limit: Int
    private var waiters: [(id: UUID, continuation: CheckedContinuation<Void, Error>)] = []
    // IDs cancelled before their continuation was registered (onCancel can
    // fire ahead of the withCheckedThrowingContinuation body).
    private var cancelledIDs: Set<UUID> = []

    init(limit: Int) {
        let normalized = max(1, limit)
        self.limit = normalized
        self.available = normalized
    }

    func setLimit(_ newLimit: Int) {
        let normalized = max(1, newLimit)
        limit = normalized
        available = min(available, normalized)
        flushQueue()
    }

    var hasWaiters: Bool { !waiters.isEmpty }

    /// Waits for a permit. Throws CancellationError if the task is cancelled
    /// before a permit is granted; a cancelled waiter never consumes a permit.
    func acquire() async throws {
        try Task.checkCancellation()
        if available > 0 {
            available -= 1
            return
        }

        let id = UUID()
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
                if cancelledIDs.remove(id) != nil {
                    continuation.resume(throwing: CancellationError())
                } else {
                    waiters.append((id, continuation))
                }
            }
        } onCancel: {
            Task { await self.cancelWaiter(id) }
        }
    }

    func release() {
        if let waiter = waiters.first {
            waiters.removeFirst()
            waiter.continuation.resume()
            return
        }
        available = min(available + 1, limit)
    }

    private func cancelWaiter(_ id: UUID) {
        if let index = waiters.firstIndex(where: { $0.id == id }) {
            let waiter = waiters.remove(at: index)
            waiter.continuation.resume(throwing: CancellationError())
        } else {
            cancelledIDs.insert(id)
        }
    }

    private func flushQueue() {
        while available > 0, !waiters.isEmpty {
            let waiter = waiters.removeFirst()
            available -= 1
            waiter.continuation.resume()
        }
    }
}
