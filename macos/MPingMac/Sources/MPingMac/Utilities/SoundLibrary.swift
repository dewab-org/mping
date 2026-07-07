import AppKit

enum SoundLibrary {
    static let availableNames: [String] = {
        // Common system alert sounds.
        let defaults = ["Basso", "Funk", "Glass", "Hero", "Ping", "Pop", "Purr", "Sosumi", "Submarine", "Tink"]
        return defaults.sorted()
    }()
}
