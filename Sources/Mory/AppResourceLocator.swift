import Foundation

struct AppResourceLocator {
    static func existingURL(
        from resourceRoot: URL?,
        relativePath: String,
        packageFallback: () -> URL?
    ) -> URL? {
        if let packagedURL = resourceRoot?.appendingPathComponent(relativePath),
           FileManager.default.fileExists(atPath: packagedURL.path) {
            return packagedURL
        }
        guard let fallbackURL = packageFallback(),
              FileManager.default.fileExists(atPath: fallbackURL.path) else {
            return nil
        }
        return fallbackURL
    }
}
