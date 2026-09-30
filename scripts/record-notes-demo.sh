#!/usr/bin/env bash
# Records the notes demo tour (example/notes, TestNotesTour) in a virtual X
# display and writes an animated GIF for the README.
#
# Needs Linux with WebKitGTK 4.1, Xvfb, and ffmpeg. Usage:
#   scripts/record-notes-demo.sh [out.gif]
set -euo pipefail

out=${1:-docs/assets/notes-demo.gif}
work=$(mktemp -d)
trap 'kill "${ffmpeg_pid:-}" "${xvfb_pid:-}" 2>/dev/null || true; rm -rf "$work"' EXIT

# Build the test binary first so compile time is not recorded.
CGO_ENABLED=1 go test -c -tags vitra_native -o "$work/notes.test" ./example/notes

export DISPLAY=:99
Xvfb "$DISPLAY" -screen 0 1180x760x24 -nolisten tcp &
xvfb_pid=$!
sleep 1

ffmpeg -loglevel error -y -f x11grab -draw_mouse 0 -video_size 1180x760 -framerate 20 -i "$DISPLAY" \
  -c:v libx264 -preset ultrafast -crf 18 -pix_fmt yuv420p "$work/raw.mp4" &
ffmpeg_pid=$!

# A fresh vault under a readable path; the footer shows it.
export VITRA_TOUR_VAULT=${VITRA_TOUR_VAULT:-$work/home/you/VitraNotes}
GTK_THEME=Adwaita VITRA_RECORD_TOUR=1 "$work/notes.test" -test.run '^TestNotesTour$' -test.v -test.count=1

kill -INT "$ffmpeg_pid"
wait "$ffmpeg_pid" || true
unset ffmpeg_pid

mkdir -p "$(dirname "$out")"
# Skip the blank first second (WebKit starting), then build a palette for
# a sharp, small GIF.
ffmpeg -loglevel error -y -ss 1 -i "$work/raw.mp4" -vf \
  "fps=10,scale=900:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=96:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" \
  "$out"
echo "wrote $out ($(du -h "$out" | cut -f1))"
