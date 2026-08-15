// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Mory",
    platforms: [.macOS(.v13)],
    products: [
        .executable(name: "Mory", targets: ["Mory"])
    ],
    targets: [
        .executableTarget(
            name: "Mory",
            path: "Sources/Mory",
            resources: [.copy("Web")]
        )
    ]
)
