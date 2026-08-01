// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "MPingMac",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(name: "MPingMac", targets: ["MPingMac"])
    ],
    dependencies: [],
    targets: [
        .executableTarget(
            name: "MPingMac",
            path: "Sources",
            resources: [
                .process("MPingMac/Resources")
            ]
        ),
        .testTarget(
            name: "MPingMacTests",
            dependencies: ["MPingMac"],
            path: "Tests/MPingMacTests"
        )
    ]
)
