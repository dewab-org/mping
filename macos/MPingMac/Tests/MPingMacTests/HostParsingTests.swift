import XCTest
@testable import MPingMac

@MainActor
final class HostParsingTests: XCTestCase {
    // MARK: CIDR expansion

    func testCIDRMasksBaseToNetworkBoundary() {
        let range = HostStore.cidrRange(from: "192.168.1.5/24")
        XCTAssertEqual(range?.count, 256)
        XCTAssertEqual(range?.first, "192.168.1.0")
        XCTAssertEqual(range?.last, "192.168.1.255")
    }

    func testCIDRCapMatchesDashRangeCap() {
        XCTAssertEqual(HostStore.cidrRange(from: "10.0.0.0/22")?.count, 1024)
        XCTAssertNil(HostStore.cidrRange(from: "10.0.0.0/21"), "wider than /22 must be rejected")
        XCTAssertNil(HostStore.cidrRange(from: "10.0.0.0/17"), "/17 previously expanded to 32768 hosts")
        XCTAssertNil(HostStore.cidrRange(from: "10.0.0.0/0"))
    }

    func testCIDRSingleHostAndInvalidInput() {
        XCTAssertEqual(HostStore.cidrRange(from: "10.0.0.7/32"), ["10.0.0.7"])
        XCTAssertNil(HostStore.cidrRange(from: "not-an-ip/24"))
        XCTAssertNil(HostStore.cidrRange(from: "10.0.0.0/33"))
        XCTAssertNil(HostStore.cidrRange(from: "10.0.0.0"))
        XCTAssertNil(HostStore.cidrRange(from: "10.0.0.0/8/24"))
    }

    // MARK: Dash ranges

    func testIPv4RangeExpansion() {
        XCTAssertEqual(
            HostStore.ipv4Range(from: "10.0.0.1-10.0.0.3"),
            ["10.0.0.1", "10.0.0.2", "10.0.0.3"]
        )
    }

    func testIPv4RangeShorthandLastOctet() {
        XCTAssertEqual(
            HostStore.ipv4Range(from: "10.0.0.250-252"),
            ["10.0.0.250", "10.0.0.251", "10.0.0.252"]
        )
    }

    func testIPv4RangeCapAndValidation() {
        XCTAssertEqual(HostStore.ipv4Range(from: "10.0.0.0-10.0.255.255")?.count, 1024, "huge ranges are truncated to the cap")
        XCTAssertNil(HostStore.ipv4Range(from: "10.0.0.5-10.0.0.1"), "reversed range")
        XCTAssertNil(HostStore.ipv4Range(from: "example.com"))
        XCTAssertNil(HostStore.ipv4Range(from: "host-name"), "hyphenated hostnames are not ranges")
    }

    // MARK: Host/port splitting

    func testSplitHostAndPort() {
        var result = HostStore.splitHostAndPort("example.com:8443")
        XCTAssertEqual(result.host, "example.com")
        XCTAssertEqual(result.port, 8443)

        result = HostStore.splitHostAndPort("example.com")
        XCTAssertEqual(result.host, "example.com")
        XCTAssertNil(result.port)

        result = HostStore.splitHostAndPort("[::1]:443")
        XCTAssertEqual(result.host, "::1")
        XCTAssertEqual(result.port, 443)

        result = HostStore.splitHostAndPort("[fe80::1]")
        XCTAssertEqual(result.host, "fe80::1")
        XCTAssertNil(result.port)

        // Bare IPv6 (multiple colons) must not be treated as host:port.
        result = HostStore.splitHostAndPort("fe80::1")
        XCTAssertEqual(result.host, "fe80::1")
        XCTAssertNil(result.port)

        // Out-of-range or non-numeric ports are not ports.
        result = HostStore.splitHostAndPort("example.com:99999")
        XCTAssertEqual(result.host, "example.com:99999")
        XCTAssertNil(result.port)
        result = HostStore.splitHostAndPort("example.com:http")
        XCTAssertEqual(result.host, "example.com:http")
        XCTAssertNil(result.port)
    }
}
