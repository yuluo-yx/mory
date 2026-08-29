import Foundation

struct LaunchExportRequest: Equatable {
    let source: URL
    let destination: URL
    let format: String
}

struct LaunchRequest: Equatable {
    var document: URL?
    var export: LaunchExportRequest?

    static func parse(arguments: [String], currentDirectory: URL = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)) throws -> LaunchRequest {
        guard !arguments.isEmpty else { return LaunchRequest() }
        if arguments[0] != "--mory-cli-export" {
            guard !arguments[0].hasPrefix("-") else { return LaunchRequest() }
            return LaunchRequest(document: try documentURL(arguments[0], relativeTo: currentDirectory))
        }

        var format = ""
        var output = ""
        var source = ""
        var index = 1
        while index < arguments.count {
            let argument = arguments[index]
            if argument == "--format" || argument == "--output" {
                guard index + 1 < arguments.count else { throw launchError("Missing value for \(argument)") }
                if argument == "--format" { format = arguments[index + 1] }
                else { output = arguments[index + 1] }
                index += 2
            } else if argument.hasPrefix("--format=") {
                format = String(argument.dropFirst("--format=".count))
                index += 1
            } else if argument.hasPrefix("--output=") {
                output = String(argument.dropFirst("--output=".count))
                index += 1
            } else if argument.hasPrefix("-") {
                throw launchError("Unknown desktop export argument: \(argument)")
            } else if source.isEmpty {
                source = argument
                index += 1
            } else {
                throw launchError("Desktop export accepts one source document")
            }
        }

        let normalizedFormat = format.lowercased()
        let extensions = ["html": "html", "pdf": "pdf", "png": "png", "jpeg": "jpg", "pptx": "pptx"]
        guard let expectedExtension = extensions[normalizedFormat] else {
            throw launchError("Unsupported export format: \(format)")
        }
        guard !source.isEmpty, !output.isEmpty else {
            throw launchError("Desktop export requires source and output paths")
        }
        let sourceURL = try documentURL(source, relativeTo: currentDirectory)
        let destination = absoluteURL(output, relativeTo: currentDirectory)
        guard destination.pathExtension.lowercased() == expectedExtension else {
            throw launchError("Desktop export output extension must be .\(expectedExtension)")
        }
        var isDirectory: ObjCBool = false
        guard FileManager.default.fileExists(atPath: destination.deletingLastPathComponent().path, isDirectory: &isDirectory), isDirectory.boolValue else {
            throw launchError("Desktop export output directory is unavailable")
        }
        let export = LaunchExportRequest(source: sourceURL, destination: destination, format: normalizedFormat)
        return LaunchRequest(document: sourceURL, export: export)
    }

    private static func documentURL(_ value: String, relativeTo directory: URL) throws -> URL {
        let url = absoluteURL(value, relativeTo: directory)
        let supported = Set(["md", "markdown", "mmd", "mdown", "mkd", "txt", "text"])
        var isDirectory: ObjCBool = false
        guard supported.contains(url.pathExtension.lowercased()),
              FileManager.default.fileExists(atPath: url.path, isDirectory: &isDirectory), !isDirectory.boolValue else {
            throw launchError("Document is unavailable or unsupported: \(url.path)")
        }
        return url
    }

    private static func absoluteURL(_ value: String, relativeTo directory: URL) -> URL {
        if NSString(string: value).isAbsolutePath { return URL(fileURLWithPath: value).standardizedFileURL }
        return directory.appendingPathComponent(value).standardizedFileURL
    }

    private static func launchError(_ message: String) -> NSError {
        NSError(domain: "Mory.Launch", code: 1, userInfo: [NSLocalizedDescriptionKey: message])
    }
}
