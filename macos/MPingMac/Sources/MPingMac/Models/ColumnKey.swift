enum ColumnKey: String, CaseIterable, Codable, Hashable {
    case status, host, ip, rtt, rttMin, rttAvg, rttMax, rttTrend
    case successTrend, success, failure, lastOK, loss, error
}
