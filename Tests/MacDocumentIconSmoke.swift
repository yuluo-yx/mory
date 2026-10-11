import AppKit

@main
@MainActor
enum MacDocumentIconSmoke {
    static func main() {
        do {
            guard CommandLine.arguments.count == 3 else {
                throw failure("Usage: mory-document-icon-smoke <application.app> <reference.png>")
            }
            _ = NSApplication.shared
            guard let reference = NSImage(contentsOfFile: CommandLine.arguments[2]), reference.isValid else {
                throw failure("The canonical icon must be a valid image")
            }
            let systemIcon = NSWorkspace.shared.icon(forFile: CommandLine.arguments[1])
            guard let unrelatedIcon = NSImage(named: NSImage.folderName) else {
                throw failure("The negative-control folder icon must be available")
            }
            for size in [16, 32, 64, 128, 256] {
                let actual = try pixels(of: systemIcon, size: size)
                let difference = try artworkDifference(reference: reference, actual: actual, size: size)
                print("System icon rendering at \(size)x\(size): normalized pixel error \(difference)")
                guard difference <= 0.08 else {
                    throw failure("Icon Services does not render the Mory artwork correctly at \(size)x\(size)")
                }
                let unrelated = try pixels(of: unrelatedIcon, size: size)
                guard try artworkDifference(reference: reference, actual: unrelated, size: size) > 0.08 else {
                    throw failure("The artwork check must reject an unrelated system icon at \(size)x\(size)")
                }
            }
            print("macOS system icon rendering passed: Finder list, Retina list, and preview sizes")
        } catch {
            fputs("\(error.localizedDescription)\n", stderr)
            Darwin.exit(1)
        }
    }

    private static func artworkDifference(reference: NSImage, actual: [UInt8], size: Int) throws -> Double {
        // Newer Icon Services adds a bezel and optical insets at small Finder sizes.
        // Compare central artwork at bounded scales, not the system-owned outer decoration.
        var best = Double.infinity
        let margin = size / 4
        for inset in -max(1, size / 16)...margin {
            let expected = try pixels(of: reference, size: size, inset: inset)
            var total = 0.0
            for y in margin..<(size - margin) {
                for x in margin..<(size - margin) {
                    for channel in 0..<4 {
                        let offset = (y * size + x) * 4 + channel
                        total += Double(abs(Int(expected[offset]) - Int(actual[offset])))
                    }
                }
            }
            let difference = total / Double((size - margin * 2) * (size - margin * 2) * 4 * 255)
            best = min(best, difference)
        }
        return best
    }

    private static func pixels(of image: NSImage, size: Int, inset: Int = 0) throws -> [UInt8] {
        guard let bitmap = NSBitmapImageRep(
            bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size,
            bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
            colorSpaceName: .deviceRGB, bytesPerRow: size * 4, bitsPerPixel: 32
        ), let data = bitmap.bitmapData, let context = NSGraphicsContext(bitmapImageRep: bitmap) else {
            throw failure("Cannot create a deterministic icon rendering context")
        }
        let count = bitmap.bytesPerRow * size
        data.initialize(repeating: 0, count: count)
        NSGraphicsContext.saveGraphicsState()
        defer { NSGraphicsContext.restoreGraphicsState() }
        NSGraphicsContext.current = context
        context.imageInterpolation = .high
        image.draw(in: NSRect(x: inset, y: inset, width: size - inset * 2, height: size - inset * 2), from: .zero, operation: .sourceOver, fraction: 1)
        let result = Array(UnsafeBufferPointer(start: data, count: count))
        let alpha = stride(from: 3, to: count, by: 4).reduce(0) { $0 + Int(result[$1]) }
        let footprint = min(size, size - inset * 2)
        guard alpha > footprint * footprint * 255 / 4 else { throw failure("Icon rendering is unexpectedly transparent") }
        return result
    }

    private static func failure(_ description: String) -> NSError {
        NSError(domain: "MoryIconRenderingSmoke", code: 1, userInfo: [NSLocalizedDescriptionKey: description])
    }
}
