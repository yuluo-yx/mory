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
            for size in [16, 32, 64, 128, 256] {
                let expected = try pixels(of: reference, size: size)
                let actual = try pixels(of: systemIcon, size: size)
                let difference = zip(expected, actual).reduce(0.0) { total, pair in
                    total + Double(abs(Int(pair.0) - Int(pair.1)))
                } / Double(expected.count * 255)
                print("System icon rendering at \(size)x\(size): normalized pixel error \(difference)")
                guard difference <= 0.08 else {
                    throw failure("Icon Services does not render the Mory artwork correctly at \(size)x\(size)")
                }
            }
            print("macOS system icon rendering passed: Finder list, Retina list, and preview sizes")
        } catch {
            fputs("\(error.localizedDescription)\n", stderr)
            Darwin.exit(1)
        }
    }

    private static func pixels(of image: NSImage, size: Int) throws -> [UInt8] {
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
        image.draw(in: NSRect(x: 0, y: 0, width: CGFloat(size), height: CGFloat(size)), from: .zero, operation: .sourceOver, fraction: 1)
        let result = Array(UnsafeBufferPointer(start: data, count: count))
        let alpha = stride(from: 3, to: count, by: 4).reduce(0) { $0 + Int(result[$1]) }
        guard alpha > size * size * 255 / 4 else { throw failure("Icon rendering is unexpectedly transparent") }
        return result
    }

    private static func failure(_ description: String) -> NSError {
        NSError(domain: "MoryIconRenderingSmoke", code: 1, userInfo: [NSLocalizedDescriptionKey: description])
    }
}
