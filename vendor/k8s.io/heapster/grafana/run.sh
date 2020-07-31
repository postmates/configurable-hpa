#!/bin/sh

# Allow access to dashboards without having to log in
# Export these variables so grafana picks them up
export GF_AUTH_ANONYMOUS_ENABLED=${GF_AUTH_ANONYMOUS_ENABLED:-true}
export GF_SERVER_PROTOCOL=${GF_SERVER_PROTOCOL:-http}

echo "Starting a utility program that will configure Grafana"
setup_grafana >/dev/stdout 2>/dev/stderr &

if [ ! -f /etc/grafana/grafana.ini ]; then
	touch /etc/grafana/grafana.ini
fi

echo "Starting Grafana in foreground mode"
exec /usr/sbin/grafana-server \
  --homepath=/usr/share/grafana \
  --config=/etc/grafana/grafana.ini \
  cfg:default.log.mode="console" \
  cfg:default.paths.data=/var/lib/grafana \
  cfg:default.paths.logs=/var/log/grafana
