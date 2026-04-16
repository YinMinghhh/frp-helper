#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
INPUT_DIR="${1:-$ROOT_DIR/dist/macos-arm64}"
APP_NAME="${APP_NAME:-FRP Helper}"
APP_BUNDLE_DIR="${2:-$ROOT_DIR/dist/$APP_NAME.app}"
DMG_PATH="${3:-$ROOT_DIR/dist/frp-helper-macos-arm64.dmg}"

require_file() {
  if [ ! -f "$1" ]; then
    echo "missing required file: $1" >&2
    exit 1
  fi
}

render_svg_png() {
  svg_path="$1"
  pixel_size="$2"
  target_png="$3"
  output_dir="$(dirname "$target_png")"
  rendered_png="$output_dir/$(basename "$svg_path").png"

  /usr/bin/qlmanage -t -s "$pixel_size" -o "$output_dir" "$svg_path" >/dev/null 2>&1
  mv "$rendered_png" "$target_png"
}

generate_icon_and_background() {
  asset_dir="$1"
  icon_svg="$asset_dir/app-icon.svg"
  icon_png="$asset_dir/app-icon.png"
  iconset_dir="$asset_dir/AppIcon.iconset"
  icon_icns="$asset_dir/AppIcon.icns"

  mkdir -p "$iconset_dir"

  cat > "$icon_svg" <<'SVG'
<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">
  <defs>
    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#176962"/>
      <stop offset="100%" stop-color="#0f4743"/>
    </linearGradient>
    <linearGradient id="accent" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#f2b05f"/>
      <stop offset="100%" stop-color="#d68d3c"/>
    </linearGradient>
  </defs>
  <rect x="48" y="48" width="928" height="928" rx="228" fill="url(#bg)"/>
  <path d="M308 512h290" stroke="url(#accent)" stroke-width="82" stroke-linecap="round"/>
  <path d="M558 382l130 130-130 130" fill="none" stroke="url(#accent)" stroke-width="82" stroke-linecap="round" stroke-linejoin="round"/>
</svg>
SVG

  render_svg_png "$icon_svg" 1024 "$icon_png"

  /usr/bin/sips -z 16 16 "$icon_png" --out "$iconset_dir/icon_16x16.png" >/dev/null
  /usr/bin/sips -z 32 32 "$icon_png" --out "$iconset_dir/icon_16x16@2x.png" >/dev/null
  /usr/bin/sips -z 32 32 "$icon_png" --out "$iconset_dir/icon_32x32.png" >/dev/null
  /usr/bin/sips -z 64 64 "$icon_png" --out "$iconset_dir/icon_32x32@2x.png" >/dev/null
  /usr/bin/sips -z 128 128 "$icon_png" --out "$iconset_dir/icon_128x128.png" >/dev/null
  /usr/bin/sips -z 256 256 "$icon_png" --out "$iconset_dir/icon_128x128@2x.png" >/dev/null
  /usr/bin/sips -z 256 256 "$icon_png" --out "$iconset_dir/icon_256x256.png" >/dev/null
  /usr/bin/sips -z 512 512 "$icon_png" --out "$iconset_dir/icon_256x256@2x.png" >/dev/null
  /usr/bin/sips -z 512 512 "$icon_png" --out "$iconset_dir/icon_512x512.png" >/dev/null
  cp "$icon_png" "$iconset_dir/icon_512x512@2x.png"
  /usr/bin/iconutil -c icns "$iconset_dir" -o "$icon_icns"
}

require_file "$INPUT_DIR/frp-helper"
require_file "$INPUT_DIR/access.json"
require_file "$INPUT_DIR/README-bundle.txt"

TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/frp-helper-app.XXXXXX")"
cleanup() {
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT INT TERM

ASSET_DIR="$TMP_ROOT/assets"
APP_DIR="$TMP_ROOT/$APP_NAME.app"
CONTENTS_DIR="$APP_DIR/Contents"
MACOS_DIR="$CONTENTS_DIR/MacOS"
RESOURCES_DIR="$CONTENTS_DIR/Resources"
DMG_STAGE_DIR="$TMP_ROOT/dmg-root"

mkdir -p "$MACOS_DIR" "$RESOURCES_DIR" "$DMG_STAGE_DIR"
rm -rf "$APP_BUNDLE_DIR" "$DMG_PATH"

generate_icon_and_background "$ASSET_DIR"

cp "$INPUT_DIR/frp-helper" "$MACOS_DIR/frp-helper-bin"
cp "$INPUT_DIR/access.json" "$MACOS_DIR/access.json"
cp "$INPUT_DIR/README-bundle.txt" "$RESOURCES_DIR/README-bundle.txt"
cp "$ASSET_DIR/AppIcon.icns" "$RESOURCES_DIR/AppIcon.icns"

if [ -f "$INPUT_DIR/start.command" ]; then
  cp "$INPUT_DIR/start.command" "$RESOURCES_DIR/start.command"
fi
if [ -f "$INPUT_DIR/stop.command" ]; then
  cp "$INPUT_DIR/stop.command" "$RESOURCES_DIR/stop.command"
fi

chmod 0755 "$MACOS_DIR/frp-helper-bin"
chmod 0600 "$MACOS_DIR/access.json"

cat > "$CONTENTS_DIR/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>zh_CN</string>
  <key>CFBundleDisplayName</key>
  <string>FRP Helper</string>
  <key>CFBundleExecutable</key>
  <string>FRP Helper</string>
  <key>CFBundleIconFile</key>
  <string>AppIcon.icns</string>
  <key>CFBundleIdentifier</key>
  <string>io.github.yinming.frp-helper.bundle</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>FRP Helper</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>1.0</string>
  <key>CFBundleVersion</key>
  <string>1</string>
  <key>LSApplicationCategoryType</key>
  <string>public.app-category.utilities</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST

cat > "$MACOS_DIR/FRP Helper" <<'SH'
#!/bin/sh
set -eu

APP_NAME="FRP Helper"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BUNDLE_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"
HELPER_BIN="$SCRIPT_DIR/frp-helper-bin"

prompt_dialog() {
  title="$1"
  icon="$2"
  content_path="$3"
  buttons_spec="$4"
  default_button="$5"

  /usr/bin/osascript - "$title" "$icon" "$content_path" "$buttons_spec" "$default_button" <<'OSA'
on run argv
  set dialogTitle to item 1 of argv
  set dialogIcon to item 2 of argv
  set textPath to item 3 of argv
  set buttonSpec to item 4 of argv
  set defaultName to item 5 of argv
  set messageText to do shell script "/bin/cat " & quoted form of textPath
  set AppleScript's text item delimiters to "|"
  set buttonLabels to text items of buttonSpec
  if dialogIcon is "stop" then
    return button returned of (display dialog messageText with title dialogTitle buttons buttonLabels default button defaultName with icon stop)
  else if dialogIcon is "caution" then
    return button returned of (display dialog messageText with title dialogTitle buttons buttonLabels default button defaultName with icon caution)
  else
    return button returned of (display dialog messageText with title dialogTitle buttons buttonLabels default button defaultName with icon note)
  end if
end run
OSA
}

copy_to_clipboard() {
  content_path="$1"
  if command -v pbcopy >/dev/null 2>&1; then
    /bin/cat "$content_path" | pbcopy || true
  fi
}

build_endpoint_summary() {
  heading="$1"
  footer="$2"
  summary_path="$3"
  clipboard_path="$4"

  endpoint_file="$(mktemp "${TMPDIR:-/tmp}/frp-helper-endpoints.XXXXXX")"
  body_file="$(mktemp "${TMPDIR:-/tmp}/frp-helper-endpoints-body.XXXXXX")"
  rm -f "$clipboard_path"

  if ! "$HELPER_BIN" endpoints >"$endpoint_file" 2>&1; then
    cp "$endpoint_file" "$summary_path"
    rm -f "$endpoint_file" "$body_file"
    return 1
  fi

  : >"$clipboard_path"
  if ! awk -F '  +' -v commands="$clipboard_path" '
    NR == 1 { next }
    NF >= 6 {
      if (count > 0) {
        printf "\n"
      }
      printf "%s\n%s\n", $2, $6
      print $6 >> commands
      count++
    }
    END {
      if (count == 0) {
        exit 1
      }
    }
  ' "$endpoint_file" >"$body_file"; then
    cp "$endpoint_file" "$summary_path"
    rm -f "$endpoint_file" "$body_file"
    return 1
  fi

  {
    printf '%s\n\n' "$heading"
    printf '访问入口：\n\n'
    cat "$body_file"
    if [ -n "$footer" ]; then
      printf '\n%s\n' "$footer"
    fi
  } >"$summary_path"

  rm -f "$endpoint_file" "$body_file"
  return 0
}

build_running_summary() {
  summary_path="$1"
  clipboard_path="$2"

  if build_endpoint_summary "FRP Helper 已在运行。" "访问命令已复制到剪贴板。" "$summary_path" "$clipboard_path"; then
    copy_to_clipboard "$clipboard_path"
    return 0
  fi

  cp "$status_file" "$summary_path"
  return 0
}

build_stopped_summary() {
  summary_path="$1"

  cat > "$summary_path" <<'EOF'
FRP Helper 当前未运行。

点击“启动”即可重新连接。
EOF
}

is_port_conflict_error() {
  log_path="$1"
  /usr/bin/grep -Eiq 'bindPort [0-9]+ is unavailable|address already in use|local bindPort is already in use|port already used|bind failed' "$log_path"
}

extract_conflict_port() {
  log_path="$1"

  port="$(/usr/bin/grep -Eo 'bindPort [0-9]+' "$log_path" | /usr/bin/awk '{print $2; exit}' || true)"
  if [ -n "$port" ]; then
    printf '%s\n' "$port"
    return 0
  fi

  port="$(/usr/bin/grep -Eo '127\\.0\\.0\\.1:[0-9]+' "$log_path" | /usr/bin/awk -F: '{print $2; exit}' || true)"
  if [ -n "$port" ]; then
    printf '%s\n' "$port"
    return 0
  fi

  return 1
}

build_port_conflict_summary() {
  log_path="$1"
  summary_path="$2"

  if ! is_port_conflict_error "$log_path"; then
    return 1
  fi

  port="$(extract_conflict_port "$log_path" || true)"
  if [ -z "$port" ]; then
    cat > "$summary_path" <<'EOF'
本地监听端口已被其他程序占用。

请先关闭占用端口的程序，或者修改配置里的 bindPort 后再重试。
EOF
    return 0
  fi

  pids="$(/usr/sbin/lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null | /usr/bin/paste -sd ' ' - || true)"

  {
    printf '本地端口 %s 已被其他程序占用。\n\n' "$port"
    printf '请先关闭占用该端口的程序，或者修改配置里的 bindPort。\n\n'
    printf '查看占用进程：\n'
    printf 'lsof -nP -iTCP:%s -sTCP:LISTEN\n' "$port"
    if [ -n "$pids" ]; then
      printf '\n结束占用进程：\n'
      for pid in $pids; do
        printf 'kill -9 %s\n' "$pid"
      done
    fi
    printf '\n处理完成后，点击“尝试重启”。\n'
  } >"$summary_path"

  return 0
}

if printf '%s' "$BUNDLE_DIR" | /usr/bin/grep -q '^/Volumes/'; then
  warn_file="$(mktemp "${TMPDIR:-/tmp}/frp-helper-warning.XXXXXX")"
  trap 'rm -f "$warn_file"' EXIT INT TERM
  cat > "$warn_file" <<'EOF'
请先把 FRP Helper.app 拖到 Applications，再打开。

否则开机启动会指向临时挂载的 DMG 路径。
EOF
  prompt_dialog "$APP_NAME" caution "$warn_file" "关闭" "关闭" >/dev/null
  exit 0
fi

status_file="$(mktemp "${TMPDIR:-/tmp}/frp-helper-status.XXXXXX")"
output_file="$(mktemp "${TMPDIR:-/tmp}/frp-helper-output.XXXXXX")"
summary_file="$(mktemp "${TMPDIR:-/tmp}/frp-helper-summary.XXXXXX")"
clipboard_file="$(mktemp "${TMPDIR:-/tmp}/frp-helper-clipboard.XXXXXX")"
cleanup() {
  rm -f "$status_file" "$output_file" "$summary_file" "$clipboard_file"
}
trap cleanup EXIT INT TERM

is_running() {
  "$HELPER_BIN" status >"$status_file" 2>&1 || true
  /usr/bin/grep -q '^running: true' "$status_file"
}

attempt_start=1

while :; do
  if is_running; then
    build_running_summary "$summary_file" "$clipboard_file"
    action="$(prompt_dialog "$APP_NAME" note "$summary_file" "关闭|停止" "关闭")"
    case "$action" in
      "停止")
        if "$HELPER_BIN" stop >"$output_file" 2>&1; then
          attempt_start=0
          continue
        fi
        cp "$output_file" "$summary_file"
        action="$(prompt_dialog "$APP_NAME" stop "$summary_file" "关闭|返回" "返回")"
        case "$action" in
          "返回")
            continue
            ;;
          *)
            exit 0
            ;;
        esac
        ;;
      *)
        exit 0
        ;;
    esac
  fi

  if [ "$attempt_start" -eq 1 ]; then
    attempt_start=0
    if "$HELPER_BIN" run --enable-startup >"$output_file" 2>&1; then
      continue
    fi

    if build_port_conflict_summary "$output_file" "$summary_file"; then
      action="$(prompt_dialog "$APP_NAME" caution "$summary_file" "关闭|尝试重启" "尝试重启")"
    else
      cp "$output_file" "$summary_file"
      action="$(prompt_dialog "$APP_NAME" stop "$summary_file" "关闭|尝试重启" "尝试重启")"
    fi

    case "$action" in
      "尝试重启")
        attempt_start=1
        continue
        ;;
      *)
        exit 0
        ;;
    esac
  fi

  build_stopped_summary "$summary_file"
  action="$(prompt_dialog "$APP_NAME" note "$summary_file" "关闭|启动" "启动")"
  case "$action" in
    "启动")
      attempt_start=1
      ;;
    *)
      exit 0
      ;;
  esac
done
SH

chmod 0755 "$MACOS_DIR/FRP Helper"

if /usr/bin/codesign --force --deep --sign - "$APP_DIR" >/dev/null 2>&1; then
  :
fi

cp -R "$APP_DIR" "$APP_BUNDLE_DIR"

cp -R "$APP_BUNDLE_DIR" "$DMG_STAGE_DIR/$APP_NAME.app"
ln -s /Applications "$DMG_STAGE_DIR/Applications"
cat > "$DMG_STAGE_DIR/README.txt" <<'EOF'
1. 把 FRP Helper.app 拖到 Applications。
2. 首次运行会自动安装并启动所需组件。
EOF

/usr/bin/hdiutil create \
  -volname "$APP_NAME" \
  -srcfolder "$DMG_STAGE_DIR" \
  -format UDZO \
  -ov \
  "$DMG_PATH" >/dev/null

echo "App: $APP_BUNDLE_DIR"
echo "DMG: $DMG_PATH"
