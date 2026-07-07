import Foundation

actor ConcurrencyGate {
    private var available: Int
    private var limit: Int
    private var waiters: [CheckedContinuation<Void, Never>] = []

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

    func acquire() async {
        if available > 0 {
            available -= 1
            return
        }

        await withCheckedContinuation { continuation in
            waiters.append(continuation)
        }
    }

    func release() {
        if let waiter = waiters.first {
            waiters.removeFirst()
            waiter.resume()
            return
        }
        available = min(available + 1, limit)
    }

    private func flushQueue() {
        while available > 0, !waiters.isEmpty {
            let waiter = waiters.removeFirst()
            available -= 1
            waiter.resume()
        }
    }
}
