import SwiftUI

struct PingHistoryPanel: View {
    let host: Host
    var onClose: () -> Void
    var onExport: () -> Void

    var body: some View {
        VStack(spacing: 8) {
            HStack {
                Text("Ping History for \(host.displayName)")
                    .font(.headline)
                Spacer()
                Button {
                    onExport()
                } label: {
                    Label("Export CSV", systemImage: "square.and.arrow.up")
                }
                .buttonStyle(.borderless)
                Button {
                    onClose()
                } label: {
                    Label("Close", systemImage: "xmark.circle")
                }
                .buttonStyle(.borderless)
            }
            Table(host.samples) {
                TableColumn("Time") { sample in
                    Text(sample.timestamp, style: .time)
                        .font(.caption.monospacedDigit())
                }
                TableColumn("Status") { sample in
                    Label(sample.success ? "Success" : "Failure", systemImage: sample.success ? "checkmark.circle.fill" : "xmark.circle.fill")
                        .foregroundStyle(sample.success ? .green : .red)
                        .font(.caption)
                }
                TableColumn("RTT") { sample in
                    Text(sample.rttMilliseconds.map { String(format: "%.1f ms", $0) } ?? "—")
                        .font(.caption.monospacedDigit())
                }
                TableColumn("Error") { sample in
                    Text(sample.error ?? "—")
                        .font(.caption)
                        .lineLimit(1)
                        .foregroundColor(sample.error == nil ? .secondary : .red)
                        .help(sample.error ?? "")
                }
            }
            .frame(minHeight: 140, maxHeight: 220)
        }
        .padding()
        .background(.thinMaterial)
    }
}
