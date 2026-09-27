#!/bin/bash
# FaultIQ Screenshot + Demo GIF Capture Script
# Usage: bash scripts/capture-screenshots.sh
# Requirements: docker, ffmpeg, screencapture (macOS) or import (Linux)

set -e

SCREENSHOT_DIR="docs/screenshots"
DEMO_GIF="docs/demo.gif"
DEMO_MP4="/tmp/faultiq-demo.mp4"

mkdir -p "$SCREENSHOT_DIR"

echo "=== FaultIQ Screenshot Capture ==="
echo ""

# 1. Check if stack is running
echo "[1/6] Checking if FaultIQ stack is running..."
if ! docker compose ps | grep -q "healthy"; then
    echo "  Stack not running. Starting..."
    docker compose up -d
    echo "  Waiting for services to be healthy (60s)..."
    sleep 60
fi

# 2. Wait for frontend to be ready
echo "[2/6] Waiting for frontend to be ready..."
for i in {1..30}; do
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:4001 | grep -q "200"; then
        echo "  Frontend is ready!"
        break
    fi
    echo "  Waiting... ($i/30)"
    sleep 2
done

# 3. Open browser
echo "[3/6] Opening browser..."
open http://localhost:4001 2>/dev/null || true
sleep 3

# 4. Take screenshots
echo "[4/6] Taking screenshots..."

# Dashboard - Service Graph
echo "  Capturing dashboard..."
screencapture -x "$SCREENSHOT_DIR/dashboard-graph.png" 2>/dev/null || \
    import -window root "$SCREENSHOT_DIR/dashboard-graph.png" 2>/dev/null || \
    echo "  WARNING: Could not capture dashboard screenshot"

# Demo Page
echo "  Capturing demo page..."
open http://localhost:4001/dashboard/demo 2>/dev/null || true
sleep 2
screencapture -x "$SCREENSHOT_DIR/demo-page.png" 2>/dev/null || \
    import -window root "$SCREENSHOT_DIR/demo-page.png" 2>/dev/null || \
    echo "  WARNING: Could not capture demo page screenshot"

# 5. Record demo video
echo "[5/6] Recording demo video (30s)..."
echo "  Please interact with the demo page to show the fault cascade..."
echo "  Recording will start in 5 seconds..."
sleep 5

# Record screen (macOS)
screencapture -x -v "$DEMO_MP4" 2>/dev/null &
RECORD_PID=$!

echo "  Recording... (30 seconds)"
echo "  Tip: Click 'Simulate Fault Cascade' and watch the graph turn red"
sleep 30

# Stop recording
kill $RECORD_PID 2>/dev/null || true
sleep 2

# 6. Convert to GIF
echo "[6/6] Converting to GIF..."
if [ -f "$DEMO_MP4" ]; then
    ffmpeg -y -i "$DEMO_MP4" -vf "fps=10,scale=800:-1:flags=lanczos" "$DEMO_GIF" 2>/dev/null
    echo "  Demo GIF created: $DEMO_GIF"
    rm -f "$DEMO_MP4"
else
    echo "  WARNING: No video file found, skipping GIF creation"
fi

echo ""
echo "=== Capture Complete ==="
echo "Screenshots saved to: $SCREENSHOT_DIR/"
echo "Demo GIF saved to: $DEMO_GIF"
echo ""
echo "Next steps:"
echo "1. Review the screenshots"
echo "2. Uncomment the screenshot lines in README.md"
echo "3. Commit and push: git add docs/ && git commit -m 'Add screenshots and demo GIF'"
