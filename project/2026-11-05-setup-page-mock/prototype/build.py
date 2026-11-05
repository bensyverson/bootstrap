#!/usr/bin/env python3
"""Builds the standalone prototype: inlines the subset fonts and card images into local.html.

To view it: python3 build.py, then open local.html (it loads Three.js from a CDN).
"""
import base64
from pathlib import Path

here = Path(__file__).parent
b64 = lambda p: base64.b64encode((here / p).read_bytes()).decode()
page = (here / "template.html").read_text()
for token, name in (("__REGULAR__", "Regular"), ("__MEDIUM__", "Medium")):
    page = page.replace(token, b64(f"fonts/Grand-{name}.subset.woff2"))
for name in ("jobs", "demo", "demo2", "design"):
    page = page.replace(f"__IMG_{name}__", b64(f"assets/{name}.jpg"))
head = '<!doctype html><html><head><meta charset=utf-8><meta name=viewport content="width=device-width,initial-scale=1"></head><body>'
(here / "local.html").write_text(head + page + "</body></html>")
