# Scripts

## launchd Setup (macOS Daily Scheduling)

The `com.gitstreams.plist` file configures macOS to run gitstreams daily at 9:00
AM.

### Installation

1. **Build and install the binary:**

   ```bash
   go build -o gitstreams .
   sudo cp gitstreams /usr/local/bin/
   ```

   Or install to a custom location and update the plist accordingly.

2. **Set up your GitHub token:**

   The plist doesn't include the token for security reasons. Create a wrapper
   script:

   ```bash
   cat > ~/.local/bin/gitstreams-wrapper <<'EOF'
   #!/bin/bash
   export GITHUB_TOKEN="$(security find-generic-password -s gitstreams -w)"
   /usr/local/bin/gitstreams
   EOF
   chmod +x ~/.local/bin/gitstreams-wrapper
   ```

   Store your token in Keychain:

   ```bash
   security add-generic-password -s gitstreams -a "$USER" -w "your_github_token"
   ```

   Then update the plist to use the wrapper script instead.

3. **Copy the plist to LaunchAgents:**

   ```bash
   cp scripts/com.gitstreams.plist ~/Library/LaunchAgents/
   ```

4. **Load the agent:**

   ```bash
   launchctl load ~/Library/LaunchAgents/com.gitstreams.plist
   ```

### Management Commands

**Check status:**

```bash
launchctl list | grep gitstreams
```

**Run manually (for testing):**

```bash
launchctl start com.gitstreams
```

**View logs:**

```bash
tail -f /tmp/gitstreams.stdout.log
tail -f /tmp/gitstreams.stderr.log
```

**Unload (stop scheduling):**

```bash
launchctl unload ~/Library/LaunchAgents/com.gitstreams.plist
```

**Reload after changes:**

```bash
launchctl unload ~/Library/LaunchAgents/com.gitstreams.plist
launchctl load ~/Library/LaunchAgents/com.gitstreams.plist
```

### Customization

Edit `com.gitstreams.plist` to change:

- **Run time:** Modify `StartCalendarInterval` (Hour/Minute)
- **Binary path:** Update the path in `ProgramArguments`
- **Log locations:** Change `StandardOutPath` and `StandardErrorPath`

### Troubleshooting

**Agent not running:**

```bash
# Check for errors
launchctl list com.gitstreams
# Exit code meanings: 0 = success, non-zero = check logs
```

**Permission issues:**

```bash
# Ensure binary is executable
chmod +x /usr/local/bin/gitstreams
```

**Token not found:**

Verify the keychain entry exists:

```bash
security find-generic-password -s gitstreams
```
