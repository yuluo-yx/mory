import CoreGraphics
import Foundation

enum PDFPaginator {
    /// 将 WebKit 生成的长页 PDF 切分为标准纸张，避免在主线程执行分页和文件写入。
    nonisolated static func write(_ data: Data, to destination: URL, paperSize: CGSize) throws {
        guard let provider = CGDataProvider(data: data as CFData),
              let document = CGPDFDocument(provider),
              let sourcePage = document.page(at: 1) else {
            throw NSError(
                domain: "Mory.Export",
                code: 4,
                userInfo: [NSLocalizedDescriptionKey: "无法读取 WebKit 生成的 PDF 数据。"]
            )
        }

        var mediaBox = CGRect(origin: .zero, size: paperSize)
        guard let consumer = CGDataConsumer(url: destination as CFURL),
              let context = CGContext(consumer: consumer, mediaBox: &mediaBox, nil) else {
            throw NSError(
                domain: "Mory.Export",
                code: 5,
                userInfo: [NSLocalizedDescriptionKey: "无法创建 PDF 文件。"]
            )
        }

        let sourceBox = sourcePage.getBoxRect(.mediaBox)
        let printable = CGRect(x: 32, y: 28, width: paperSize.width - 64, height: paperSize.height - 56)
        let scale = printable.width / max(1, sourceBox.width)
        let sourceSliceHeight = printable.height / scale
        let pageCount = max(1, Int(ceil(sourceBox.height / sourceSliceHeight)))

        for index in 0..<pageCount {
            context.beginPDFPage(nil)
            context.saveGState()
            context.clip(to: printable)
            context.translateBy(x: printable.minX, y: printable.minY)
            context.scaleBy(x: scale, y: scale)
            let lowerSourceY = sourceBox.height - CGFloat(index + 1) * sourceSliceHeight
            context.translateBy(x: -sourceBox.minX, y: -sourceBox.minY - lowerSourceY)
            context.drawPDFPage(sourcePage)
            context.restoreGState()
            context.endPDFPage()
        }

        context.closePDF()
    }
}
