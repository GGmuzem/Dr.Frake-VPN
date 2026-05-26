# Experimental XHTTP profiles

These profiles are intentionally separate from the default self-hosted
bootstrap. Keep AmneziaWG and the current TCP Vision profile as fallbacks while
testing XHTTP.

## Test order

1. `xhttp-tls-cdn-origin-server.json`
   - Use when you can terminate TLS on the origin or behind a CDN/reverse proxy.
   - Uses `network=xhttp`, `security=tls`, `mode=packet-up`.
   - The client can try H3 with `alpn=["h3"]`.

2. `xhttp-tls-h3-client-outbound.json`
   - Client outbound snippet for the CDN/H3 test.
   - Use CDN IP or preferred IP as `address`, but keep real domain in `serverName`.

## Required placeholders

- `$XRAY_CLIENT_ID`
- `$XRAY_SITE_NAME`
- `$XRAY_XHTTP_PATH`, for example `/api/v1/events`
- `$XRAY_TLS_CERT_FILE`, `$XRAY_TLS_KEY_FILE` for TLS origin profile

## Notes

- Do not use `xtls-rprx-vision` with XHTTP.
- `packet-up` is the first XHTTP mode to test against middleboxes/CDN.
- For CDN/H3, the client uses `alpn=["h3"]`; the origin can still serve H1/H2
  behind the CDN depending on the CDN behavior.
