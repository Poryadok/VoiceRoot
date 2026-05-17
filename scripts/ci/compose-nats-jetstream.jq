# stdin: `docker compose config --format json`
(.services.nats != null)
and ((.services.nats.command // []) | index("-js") != null)
