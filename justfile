# Bookmarkd justfile

# Default recipe
default:
    @just --list

# Build the bookmarkd binary
build:
    go build -o bookmarkd .

# Run tests
test:
    go test ./...

# Install bookmarkd to ~/.local/bin
install: build
    mkdir -p ~/.local/bin
    cp bookmarkd ~/.local/bin/bookmarkd
    chmod +x ~/.local/bin/bookmarkd
    @echo "Installed bookmarkd to ~/.local/bin/bookmarkd"
    @echo "Make sure ~/.local/bin is in your PATH"

# Uninstall bookmarkd from ~/.local/bin
uninstall:
    rm -f ~/.local/bin/bookmarkd
    @echo "Removed bookmarkd from ~/.local/bin"

# Install launchctl service (macOS)
install-service: install
    #!/usr/bin/env bash
    set -euo pipefail

    PLIST_PATH="$HOME/Library/LaunchAgents/net.mulliken.bookmarkd.plist"
    DATA_DIR="$HOME/.local/share/bookmarkd"
    LOG_DIR="$HOME/.local/share/bookmarkd/logs"

    # Create data and log directories
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"

    # Create the plist file
    cat > "$PLIST_PATH" << EOF
    <?xml version="1.0" encoding="UTF-8"?>
    <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
    <plist version="1.0">
    <dict>
        <key>Label</key>
        <string>net.mulliken.bookmarkd</string>
        <key>ProgramArguments</key>
        <array>
            <string>$HOME/.local/bin/bookmarkd</string>
            <string>--db</string>
            <string>$DATA_DIR/bookmarkd.db</string>
            <string>--host</string>
            <string>localhost</string>
            <string>--port</string>
            <string>8080</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
        <key>KeepAlive</key>
        <true/>
        <key>StandardOutPath</key>
        <string>$LOG_DIR/bookmarkd.log</string>
        <key>StandardErrorPath</key>
        <string>$LOG_DIR/bookmarkd.err</string>
        <key>WorkingDirectory</key>
        <string>$DATA_DIR</string>
    </dict>
    </plist>
    EOF

    echo "Created launchctl plist at $PLIST_PATH"
    echo "Data directory: $DATA_DIR"
    echo "Log directory: $LOG_DIR"
    echo ""
    echo "To start the service now, run:"
    echo "  launchctl load $PLIST_PATH"
    echo ""
    echo "To start on next login, it will load automatically."

# Uninstall launchctl service (macOS)
uninstall-service:
    #!/usr/bin/env bash
    set -euo pipefail

    PLIST_PATH="$HOME/Library/LaunchAgents/net.mulliken.bookmarkd.plist"

    if [ -f "$PLIST_PATH" ]; then
        launchctl unload "$PLIST_PATH" 2>/dev/null || true
        rm -f "$PLIST_PATH"
        echo "Removed launchctl service"
    else
        echo "Service plist not found at $PLIST_PATH"
    fi

# Start the launchctl service
start-service:
    launchctl load ~/Library/LaunchAgents/net.mulliken.bookmarkd.plist

# Stop the launchctl service
stop-service:
    launchctl unload ~/Library/LaunchAgents/net.mulliken.bookmarkd.plist

# Restart the launchctl service
restart-service: stop-service start-service

# Show service status
status-service:
    @launchctl list | grep bookmarkd || echo "Service not running"

# View service logs
logs:
    tail -f ~/.local/share/bookmarkd/logs/bookmarkd.log

# View service error logs
logs-err:
    tail -f ~/.local/share/bookmarkd/logs/bookmarkd.err

# Clean build artifacts
clean:
    rm -f bookmarkd
