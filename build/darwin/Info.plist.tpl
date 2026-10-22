<?xml version="1.0" encoding="UTF-8"?>
<!--
  Rendered by build/darwin/bundle.sh (@VERSION@ -> the release version).

  Two executables share this plist: lich, the pure-Go binary LaunchServices
  starts, which never touches AppKit, and lich-native, the window, which does.
  macOS reads a process's bundle off its executable's path, so the window in
  Contents/MacOS is Lich.app to the Dock, to Cmd-Tab and to the menu bar: its
  icon is CFBundleIconFile and its name CFBundleName.

  CFBundleIdentifier predates the lichdotdev org and stays: macOS keys the
  notification permission a user granted on it, so a new one would ask again
  and drop whatever they set in System Settings › Notifications.
-->
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>lich</string>
  <key>CFBundleIdentifier</key>
  <string>com.github.omartelo.lich</string>
  <key>CFBundleName</key>
  <string>lich</string>
  <key>CFBundleDisplayName</key>
  <string>lich</string>
  <key>CFBundleIconFile</key>
  <string>lich</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>@VERSION@</string>
  <key>CFBundleVersion</key>
  <string>@VERSION@</string>
  <key>LSMinimumSystemVersion</key>
  <string>13.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
