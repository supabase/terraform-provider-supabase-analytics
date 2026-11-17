import json, os, random, sys
from datetime import datetime, timezone, timedelta
random.seed(7)
now = datetime.now(timezone.utc)
iso = lambda d: d.isoformat().replace("+00:00", "Z")
kind = sys.argv[1]
ev = []
if kind == "edge":
    fns = [("stripe-webhook", 0.35), ("send-email", 0.12), ("hello", 0.0), ("resize-image", 0.01)]
    for minutes in range(0, 360, 2):
        for fn, err in fns:
            n = 3 if minutes < 30 else 1
            for _ in range(n):
                r = err * (3 if minutes < 30 and fn == "stripe-webhook" else 1)
                status = random.choice([500, 502, 504] if fn != "send-email" else [503]) if random.random() < r else 200
                ms = random.randint(40, 400) if status == 200 else random.randint(900, 10000)
                ts = now - timedelta(minutes=minutes, seconds=random.randint(0, 119))
                ev.append({"timestamp": iso(ts),
                           "message": f"POST | {status} | https://default.supabase.co/functions/v1/{fn}",
                           "metadata": {"function_id": fn, "status_code": status, "method": "POST",
                                        "execution_time_ms": ms, "region": "us-east-1"}})
elif kind == "fnlogs":
    for i in range(40):
        ts = now - timedelta(seconds=random.randint(0, 170))
        fn, level, msg = random.choice([
            ("stripe-webhook", "error", "Error: Stripe API request timed out after 10000ms"),
            ("send-email", "error", "Error: SMTP connection refused (smtp.sendgrid.net:587)"),
            ("stripe-webhook", "info", "Processed event checkout.session.completed"),
            ("hello", "info", "Listening on http://localhost:9999/"),
            ("hello", "log", "Hello from Functions!"),
        ])
        ev.append({"timestamp": iso(ts), "message": msg,
                   "metadata": {"function_id": fn, "level": level, "event_type": "Log"}})
elif kind == "advisors":
    for lint in json.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "advisors.json"))):
        m = lint["metadata"]
        lint = dict(lint, facing="EXTERNAL",
                    cache_key=f"{lint['name']}_{m.get('schema','auth')}_{m.get('name', m.get('entity'))}")
        ev.append({"timestamp": iso(now - timedelta(minutes=1)), "message": lint["detail"], **lint})
print(json.dumps({"batch": ev}))
