# Rendered by .github/workflows/release.yml (@VERSION@ -> tag, checksums from the
# release's checksums.txt) and pushed to the lichdotdev/homebrew-tap repository
# twice: Casks/lich.rb, every release, and Casks/lich@beta.rb, every release and
# pre-release, the cask the beta channel updates through. Edit this template,
# never the tap copies.
#
# A cask, not a formula: only a cask installs an .app into /Applications, which
# is what puts lich in Launchpad, in Spotlight and behind its own icon. The
# binary stanza keeps `lich` on PATH, so the one install still serves both.
cask "@TOKEN@" do
  arch arm: "arm64", intel: "amd64"

  version "@VERSION@"
  sha256 arm: "@SHA_ARM64@", intel: "@SHA_AMD64@"

  url "https://github.com/lichdotdev/lich/releases/download/v#{version}/lich-v#{version}-darwin-#{arch}.zip"
  name "lich"
  desc "Run coding agents side by side, each on its own branch"
  homepage "https://github.com/lichdotdev/lich"

  depends_on macos: ">= :ventura"
  # Both casks install the same Lich.app.
  conflicts_with cask: "@OTHER@"

  app "Lich.app"
  binary "#{appdir}/Lich.app/Contents/MacOS/lich"

  # lich is ad-hoc signed and never notarized, so Gatekeeper refuses a
  # quarantined copy outright. The archive is pinned by the checksums above and
  # comes from the project's own release — that is the trust this trades on.
  postflight do
    system_command "/usr/bin/xattr",
                   args: ["-dr", "com.apple.quarantine", "#{appdir}/Lich.app"]
  end

  zap trash: [
    "~/Library/Application Support/lich",
  ]
end
