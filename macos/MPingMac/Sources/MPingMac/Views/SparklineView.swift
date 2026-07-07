import SwiftUI

struct SparklineView: View {
    let values: [Double]
    var rangeOverride: ClosedRange<Double>? = nil
    var color: Color = .accentColor
    var gradientRange: ClosedRange<Double>? = nil
    var fill: Bool = false

    private var normalizedPoints: [CGPoint] {
        guard values.count > 1 else { return [] }
        let maxValue = rangeOverride?.upperBound ?? (values.max() ?? 1)
        let minValue = rangeOverride?.lowerBound ?? (values.min() ?? 0)
        let range = max(maxValue - minValue, 0.0001)

        return values.enumerated().map { index, value in
            let x = Double(index) / Double(values.count - 1)
            let y = (value - minValue) / range
            return CGPoint(x: x, y: 1 - y)
        }
    }

    private var strokeColor: Color {
        guard let gradientRange else { return color }
        guard let value = values.last else { return color }
        return Self.gradientColor(for: value, in: gradientRange)
    }

    var body: some View {
        GeometryReader { geo in
            if normalizedPoints.count > 1 {
                let linePath = Path { path in
                    let start = normalizedPoints[0]
                    path.move(to: CGPoint(x: start.x * geo.size.width, y: start.y * geo.size.height))
                    for point in normalizedPoints.dropFirst() {
                        path.addLine(to: CGPoint(x: point.x * geo.size.width, y: point.y * geo.size.height))
                    }
                }
                ZStack {
                    if fill {
                        let fillPath = Path { path in
                            let start = normalizedPoints[0]
                            path.move(to: CGPoint(x: start.x * geo.size.width, y: start.y * geo.size.height))
                            for point in normalizedPoints.dropFirst() {
                                path.addLine(to: CGPoint(x: point.x * geo.size.width, y: point.y * geo.size.height))
                            }
                            path.addLine(to: CGPoint(x: geo.size.width, y: geo.size.height))
                            path.addLine(to: CGPoint(x: 0, y: geo.size.height))
                            path.closeSubpath()
                        }
                        fillPath.fill(strokeColor.opacity(0.2))
                    }
                    linePath.stroke(strokeColor, lineWidth: 1.5)
                }
            } else {
                Text("—")
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .center)
            }
        }
        .frame(height: 28)
    }

    private static func gradientColor(for value: Double, in range: ClosedRange<Double>) -> Color {
        let clamped = min(max(value, range.lowerBound), range.upperBound)
        let position = (clamped - range.lowerBound) / (range.upperBound - range.lowerBound)
        if position <= 0.5 {
            let t = position / 0.5
            return Color(red: 1.0, green: t, blue: 0.0)
        } else {
            let t = (position - 0.5) / 0.5
            return Color(red: 1.0 - t, green: 1.0, blue: 0.0)
        }
    }
}
