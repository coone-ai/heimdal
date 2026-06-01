#!/bin/bash
set -e

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "Usage: $0 <version>" >&2
  exit 1
fi

declare -A PLATFORMS=(
  ["darwin-arm64"]="dist/heimdal_darwin_arm64/heimdal"
  ["darwin-amd64"]="dist/heimdal_darwin_amd64/heimdal"
  ["linux-amd64"]="dist/heimdal_linux_amd64/heimdal"
  ["linux-arm64"]="dist/heimdal_linux_arm64/heimdal"
)

echo "→ Checksumlar hesaplanıyor..."
MANIFEST_PLATFORMS=""
FIRST=true

for platform in "${!PLATFORMS[@]}"; do
  binary="${PLATFORMS[$platform]}"
  [ -f "$binary" ] || continue

  checksum=$(sha256sum "$binary" | cut -d' ' -f1)
  echo "  ✓ $platform: $checksum"

  [ "$FIRST" = true ] && FIRST=false || MANIFEST_PLATFORMS="${MANIFEST_PLATFORMS},"
  MANIFEST_PLATFORMS="${MANIFEST_PLATFORMS}
    \"${platform}\": { \"checksum\": \"${checksum}\" }"
done

cat > /tmp/manifest.json << MANIFEST
{
  "version": "${VERSION}",
  "platforms": {${MANIFEST_PLATFORMS}
  }
}
MANIFEST

echo "→ Binary'ler GCS'e yükleniyor..."
for platform in "${!PLATFORMS[@]}"; do
  binary="${PLATFORMS[$platform]}"
  [ -f "$binary" ] || continue

  gsutil -h "Cache-Control:public, max-age=31536000, immutable" \
    cp "$binary" "gs://${CDN_BUCKET}/releases/${VERSION}/${platform}/heimdal"
  echo "  ✓ ${platform}/heimdal"
done

echo "→ manifest.json yükleniyor..."
gsutil -h "Cache-Control:public, max-age=31536000, immutable" \
  cp /tmp/manifest.json "gs://${CDN_BUCKET}/releases/${VERSION}/manifest.json"

echo "→ latest güncelleniyor..."
echo -n "$VERSION" > /tmp/latest
gsutil -h "Cache-Control:no-cache" \
  cp /tmp/latest "gs://${CDN_BUCKET}/releases/latest"

echo ""
echo "✅ ${VERSION} yüklendi → https://downloads.ai-la.com/releases/${VERSION}/manifest.json"
