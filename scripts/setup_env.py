#!/usr/bin/env python3
"""Create a local development .env without printing its credentials."""
import secrets
from pathlib import Path
from urllib.parse import quote

root = Path(__file__).resolve().parents[1]
target = root / ".env"
if target.exists():
    print(".env already exists; kept unchanged.")
else:
    admin_password = secrets.token_urlsafe(24)
    app_password = secrets.token_urlsafe(24)
    text = (root / ".env.example").read_text()
    text = text.replace("POSTGRES_PASSWORD=\n", f"POSTGRES_PASSWORD={admin_password}\n")
    text = text.replace("APP_DB_PASSWORD=\n", f"APP_DB_PASSWORD={app_password}\n")
    text = text.replace("JWT_SECRET=\n", f"JWT_SECRET={secrets.token_urlsafe(48)}\n")
    test_url = f"postgres://libratrack:{quote(app_password, safe='')}@127.0.0.1:5440/libratrack10_test?sslmode=disable"
    text = text.replace("TEST_DATABASE_URL=\n", f"TEST_DATABASE_URL={test_url}\n")
    with target.open("x", encoding="utf-8") as file:
        target.chmod(0o600)
        file.write(text)
    print("Created .env with separate random local passwords; credentials are not printed.")
