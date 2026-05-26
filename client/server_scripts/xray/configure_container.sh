cd /opt/amnezia/xray

if [ -z "$XRAY_SITE_NAME" ]; then
    echo "XRAY_SITE_NAME is required" >&2
    exit 1
fi
XRAY_XHTTP_PATH="${XRAY_XHTTP_PATH:-/assets/7d91f0e4}"
case "$XRAY_XHTTP_PATH" in
    /*) ;;
    *) XRAY_XHTTP_PATH="/$XRAY_XHTTP_PATH" ;;
esac
XRAY_CLIENT_ID=$(xray uuid) && echo $XRAY_CLIENT_ID > /opt/amnezia/xray/xray_uuid.key
XRAY_SHORT_ID=$(openssl rand -hex 8) && echo $XRAY_SHORT_ID > /opt/amnezia/xray/xray_short_id.key

KEYPAIR=$(xray x25519)
LINE_NUM=1
while IFS= read -r line; do
   if [[ $LINE_NUM -gt 1 ]]
      then
           IFS=":" read FIST XRAY_PUBLIC_KEY <<< "$line"
      else
      	   LINE_NUM=$((LINE_NUM + 1))
           IFS=":" read FIST XRAY_PRIVATE_KEY <<< "$line"
      fi
done <<< "$KEYPAIR"

XRAY_PRIVATE_KEY=$(echo $XRAY_PRIVATE_KEY | tr -d ' ')
XRAY_PUBLIC_KEY=$(echo $XRAY_PUBLIC_KEY | tr -d ' ')


echo $XRAY_PUBLIC_KEY > /opt/amnezia/xray/xray_public.key
echo $XRAY_PRIVATE_KEY > /opt/amnezia/xray/xray_private.key

openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout /opt/amnezia/xray/xray_tls.key \
    -out /opt/amnezia/xray/xray_tls.crt \
    -subj "/CN=$XRAY_SITE_NAME" \
    -days 3650 >/dev/null 2>&1

cat > /opt/amnezia/xray/server.json <<EOF
{
    "log": {
        "loglevel": "error"
    },
    "inbounds": [
        {
            "port": $XRAY_SERVER_PORT,
            "protocol": "vless",
            "settings": {
                "clients": [
                    {
                        "id": "$XRAY_CLIENT_ID"
                    }
                ],
                "decryption": "none"
            },
            "streamSettings": {
                "network": "xhttp",
                "security": "tls",
                "xhttpSettings": {
                    "path": "$XRAY_XHTTP_PATH",
                    "mode": "packet-up"
                },
                "tlsSettings": {
                    "serverName": "$XRAY_SITE_NAME",
                    "alpn": [
                        "h2",
                        "http/1.1"
                    ],
                    "certificates": [
                        {
                            "certificateFile": "/opt/amnezia/xray/xray_tls.crt",
                            "keyFile": "/opt/amnezia/xray/xray_tls.key"
                        }
                    ]
                }
            },
            "sniffing": {
                "enabled": true,
                "destOverride": [
                    "http",
                    "tls",
                    "quic"
                ]
            }
        }
    ],
    "outbounds": [
        {
            "tag": "direct",
            "protocol": "freedom"
        },
        {
            "tag": "block",
            "protocol": "blackhole"
        }
    ],
    "routing": {
        "rules": [
            {
                "type": "field",
                "ip": [
                    "geoip:private"
                ],
                "outboundTag": "block"
            },
            {
                "type": "field",
                "protocol": [
                    "bittorrent"
                ],
                "outboundTag": "block"
            }
        ]
    }
}
EOF
