#!/bin/bash
# Remove macOS quarantine attribute from downloaded binaries
# Run this after extracting the ZIP: ./remove-quarantine.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Removing quarantine attribute from goPensionForecast binaries..."

for file in "$SCRIPT_DIR"/goPensionForecast-*; do
    if [ -f "$file" ]; then
        xattr -d com.apple.quarantine "$file" 2>/dev/null && echo "  Cleared: $(basename "$file")"
    fi
done

echo "Done. You can now run the binaries."
