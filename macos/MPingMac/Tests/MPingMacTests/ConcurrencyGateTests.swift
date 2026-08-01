import XCTest
@testable import MPingMac

final class ConcurrencyGateTests: XCTestCase {
    func testAcquireWithinLimitDoesNotWait() async throws {
        let gate = ConcurrencyGate(limit: 2)
        try await gate.acquire()
        try await gate.acquire()
        let waiting = await gate.hasWaiters
        XCTAssertFalse(waiting)
        await gate.release()
        await gate.release()
    }

    func testAcquireBeyondLimitWaitsUntilRelease() async throws {
        let gate = ConcurrencyGate(limit: 1)
        try await gate.acquire()

        let second = Task {
            try await gate.acquire()
            return true
        }
        try await waitUntil { await gate.hasWaiters }

        await gate.release()
        let resumed = try await second.value
        XCTAssertTrue(resumed)
        await gate.release()

        // Permit fully returned: an immediate acquire must succeed.
        try await gate.acquire()
        await gate.release()
    }

    func testCancelledWaiterThrowsAndConsumesNoPermit() async throws {
        let gate = ConcurrencyGate(limit: 1)
        try await gate.acquire()

        let waiter = Task {
            try await gate.acquire()
        }
        try await waitUntil { await gate.hasWaiters }
        waiter.cancel()

        do {
            try await waiter.value
            XCTFail("cancelled waiter should throw")
        } catch is CancellationError {
            // expected
        }

        let stillWaiting = await gate.hasWaiters
        XCTAssertFalse(stillWaiting, "cancelled waiter must be removed from the queue")

        // The cancelled waiter must not have taken the permit.
        await gate.release()
        try await gate.acquire()
        await gate.release()
    }

    func testAcquireThrowsImmediatelyWhenAlreadyCancelled() async throws {
        let gate = ConcurrencyGate(limit: 1)
        let task = Task {
            try await Task.sleep(nanoseconds: 60_000_000_000) // parked until cancelled
            try await gate.acquire()
        }
        task.cancel()
        do {
            try await task.value
            XCTFail("expected CancellationError")
        } catch is CancellationError {
            // expected
        }
        // Permit must be untouched.
        try await gate.acquire()
        await gate.release()
    }

    func testSetLimitIncreaseFlushesWaiters() async throws {
        let gate = ConcurrencyGate(limit: 1)
        try await gate.acquire()

        let second = Task {
            try await gate.acquire()
            return true
        }
        try await waitUntil { await gate.hasWaiters }

        await gate.setLimit(2)
        let resumed = try await second.value
        XCTAssertTrue(resumed)
    }

    func testLoweringLimitDrainsBeforeAdmittingWaiters() async throws {
        let gate = ConcurrencyGate(limit: 2)
        try await gate.acquire()
        try await gate.acquire()
        await gate.setLimit(1)

        let waiter = Task {
            try await gate.acquire()
            return true
        }
        try await waitUntil { await gate.hasWaiters }

        await gate.release() // 2 in flight > limit 1: shrink, waiter stays parked
        let stillWaiting = await gate.hasWaiters
        XCTAssertTrue(stillWaiting, "waiter must not be admitted while over the lowered limit")

        await gate.release() // now at the limit: permit transfers to the waiter
        let resumed = try await waiter.value
        XCTAssertTrue(resumed)
        await gate.release()
    }

    func testReleaseHandsPermitToOldestWaiter() async throws {
        let gate = ConcurrencyGate(limit: 1)
        try await gate.acquire()

        actor Order {
            var values: [Int] = []
            func append(_ v: Int) { values.append(v) }
        }
        let order = Order()

        let first = Task {
            try await gate.acquire()
            await order.append(1)
            await gate.release()
        }
        try await waitUntil { await gate.hasWaiters }
        let secondTask = Task {
            try await gate.acquire()
            await order.append(2)
            await gate.release()
        }
        try await waitUntil { await gate.hasWaiters }

        await gate.release()
        _ = try await first.value
        _ = try await secondTask.value
        let seen = await order.values
        XCTAssertEqual(seen, [1, 2], "waiters should resume in FIFO order")
    }

    private func waitUntil(timeout: TimeInterval = 2.0, _ condition: () async -> Bool) async throws {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if await condition() { return }
            try await Task.sleep(nanoseconds: 5_000_000)
        }
        XCTFail("condition not met within \(timeout)s")
    }
}
