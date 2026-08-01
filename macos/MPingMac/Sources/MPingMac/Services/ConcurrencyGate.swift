import Foundation

actor ConcurrencyGate {
    // Tracking permits in use (rather than permits free) lets setLimit take
    // effect immediately in both directions: raising the limit admits parked
    // waiters at once, lowering it drains gradually as permits are released.
    private var inUse = 0
    private var limit: Int
    private var waiters: [(id: UUID, continuation: CheckedContinuation<Void, Error>)] = []
    // IDs cancelled before their continuation was registered (onCancel can
    // fire ahead of the withCheckedThrowingContinuation body).
    private var cancelledIDs: Set<UUID> = []

    init(limit: Int) {
        self.limit = max(1, limit)
    }

    func setLimit(_ newLimit: Int) {
        limit = max(1, newLimit)
        flushQueue()
    }

    var hasWaiters: Bool { !waiters.isEmpty }

    /// Waits for a permit. Throws CancellationError if the task is cancelled
    /// before a permit is granted; a cancelled waiter never consumes a permit.
    func acquire() async throws {
        try Task.checkCancellation()
        if inUse < limit {
            inUse += 1
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
        // Hand the permit to the oldest waiter unless the limit was lowered
        // below the in-flight count, in which case concurrency must shrink.
        if inUse <= limit, let waiter = waiters.first {
            waiters.removeFirst()
            waiter.continuation.resume()
            return
        }
        inUse -= 1
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
        while inUse < limit, !waiters.isEmpty {
            let waiter = waiters.removeFirst()
            inUse += 1
            waiter.continuation.resume()
        }
    }
}
