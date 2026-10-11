#!/bin/sh
# Рендер баннеров из banner.html в JPEG 2400x1350.
set -eu
cd "$(dirname "$0")"
chrome='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
for b in hero event guide; do
	"$chrome" --headless=new --disable-gpu --hide-scrollbars --allow-file-access-from-files \
		--force-device-scale-factor=1.5 --window-size=1600,900 --virtual-time-budget=3000 \
		--screenshot="$PWD/$b.png" "file://$PWD/banner.html#$b" 2>/dev/null
	sips -s format jpeg -s formatOptions 88 "$b.png" --out "$b.jpg" >/dev/null
	rm "$b.png"
done
