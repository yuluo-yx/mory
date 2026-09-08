import Foundation

struct DocumentSaveSnapshot {
    let documentID: String
    let sourceURL: URL?
    let name: String
    let markdown: String
    let rootURL: URL

    init?(dictionary: [String: Any], rootURL: URL) {
        guard let documentID = dictionary["documentId"] as? String,
              !documentID.isEmpty,
              let name = dictionary["name"] as? String,
              let markdown = dictionary["markdown"] as? String else { return nil }
        self.documentID = documentID
        self.name = name
        self.markdown = markdown
        self.rootURL = rootURL
        let path = dictionary["path"] as? String ?? ""
        self.sourceURL = path.isEmpty ? nil : URL(fileURLWithPath: path)
    }
}
