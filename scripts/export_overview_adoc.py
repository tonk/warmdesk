#!/usr/bin/env python3
"""Export the WarmDesk Time Tracking overview as AsciiDoc.

Fetches GET /api/v1/time-entries/overview/{norm,checks,travel,vacation}
and renders the same four panels as one .adoc document.

Usage:
  python scripts/export_overview_adoc.py --year 2026
  python scripts/export_overview_adoc.py --year 2026 -o overview-2026.adoc
  python scripts/export_overview_adoc.py --url https://warmdesk.example.com --year 2026

Auth (first match wins):
  --api-key / WARMDESK_API_KEY / `pass show warmdesk/api_keys/tonk`

URL (first match wins):
  --url / WARMDESK_URL / url: in ~/.config/warmdesk/timer.yaml
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
from datetime import date
from pathlib import Path

import requests
import yaml

DEFAULT_PASS_PATH = "warmdesk/api_keys/tonk"
TIMER_YAML = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config")) / "warmdesk" / "timer.yaml"

MONTHS_NL = [
    "", "januari", "februari", "maart", "april", "mei", "juni",
    "juli", "augustus", "september", "oktober", "november", "december",
]

CHECK_TYPES = {
    "overlap": "Overlappende boekingen",
    "duration_mismatch": "Duur wijkt af van begin-/eindtijd",
    "time_format": "Afwijkend tijdformaat",
    "no_times": "Geen begin-/eindtijd",
    "missing_distance": "Reistijd zonder afstand",
    "no_contract": "Geen contract",
    "weekend_entries": "Boekingen in het weekend",
    "day_over_norm": "Dag boven de norm",
    "day_under_norm": "Dag onder de norm",
    "day_without_entries": "Normdag zonder boekingen",
}


def parse_args():
    p = argparse.ArgumentParser(description="Export WarmDesk time overview as AsciiDoc")
    p.add_argument("--url", default=None, help="WarmDesk base URL (no /api/v1)")
    p.add_argument("--api-key", default=None, help="Personal API key (X-API-Key)")
    p.add_argument("--pass-path", default=DEFAULT_PASS_PATH,
                   help=f"pass entry for the API key (default: {DEFAULT_PASS_PATH})")
    p.add_argument("--year", type=int, default=date.today().year, help="Overview year")
    p.add_argument("--user-id", type=int, default=None,
                   help="Other user id (admin / time_tracking_viewer only)")
    p.add_argument("-o", "--out", default=None,
                   help="Output .adoc path (default: stdout)")
    p.add_argument("--sections", default="norm,checks,travel,vacation",
                   help="Comma-separated sections to include")
    return p.parse_args()


def load_timer_yaml():
    if not TIMER_YAML.is_file():
        return {}
    with TIMER_YAML.open() as f:
        data = yaml.safe_load(f) or {}
    return data if isinstance(data, dict) else {}


def resolve_url(cli_url):
    if cli_url:
        return cli_url.rstrip("/")
    env = os.environ.get("WARMDESK_URL", "").strip()
    if env:
        return env.rstrip("/")
    url = (load_timer_yaml().get("url") or "").strip()
    if url:
        return url.rstrip("/")
    raise SystemExit("No URL: pass --url, set WARMDESK_URL, or configure url in "
                     f"{TIMER_YAML}")


def resolve_api_key(cli_key, pass_path):
    if cli_key:
        return cli_key.strip()
    env = os.environ.get("WARMDESK_API_KEY", "").strip()
    if env:
        return env
    try:
        out = subprocess.check_output(["pass", "show", pass_path], text=True)
    except FileNotFoundError:
        raise SystemExit("pass is not installed; pass --api-key or set WARMDESK_API_KEY")
    except subprocess.CalledProcessError as e:
        raise SystemExit(f"pass show {pass_path} failed (exit {e.returncode})")
    key = out.splitlines()[0].strip() if out.strip() else ""
    if not key:
        raise SystemExit(f"pass show {pass_path} returned an empty key")
    return key


def api_get(base, path, headers, params):
    url = f"{base}/api/v1{path}"
    r = requests.get(url, headers=headers, params=params, timeout=60)
    if not r.ok:
        try:
            body = r.json()
            msg = body.get("error", body)
        except Exception:
            msg = r.text
        raise SystemExit(f"HTTP {r.status_code} GET {path}: {msg}")
    return r.json()


def fmt_hm(minutes):
    m = int(round(minutes or 0))
    sign = "-" if m < 0 else ""
    abs_m = abs(m)
    return f"{sign}{abs_m // 60}:{abs_m % 60:02d}"


def fmt_signed_hm(minutes):
    m = int(round(minutes or 0))
    if m == 0:
        return "0:00"
    return f"+{fmt_hm(m)}" if m > 0 else fmt_hm(m)


def fmt_km(km):
    return str(round((km or 0) * 10) / 10)


def adoc_escape(s):
    """Escape cell text that would break an AsciiDoc table cell."""
    if s is None:
        return ""
    return str(s).replace("|", "\\|").replace("\n", " ")


def adoc_table(headers, rows):
    cols = ",".join(["1"] * len(headers))
    lines = [f'[cols="{cols}",options="header"]', "|==="]
    lines.append("| " + " | ".join(headers))
    for row in rows:
        lines.append("| " + " | ".join(adoc_escape(c) for c in row))
    lines.append("|===")
    return "\n".join(lines)


def render_norm(data):
    lines = [
        "== Norm",
        "",
        f"Norm gebaseerd op *{data['hours_per_week']}* uur per week "
        f"({fmt_hm(data['norm_minutes_per_day'])} per dag).",
        "",
        "[cols=\"2,1\"]",
        "|===",
        "| Norm | " + fmt_hm(data["total"]["norm_minutes"]),
        "| Geboekt | " + fmt_hm(data["total"]["booked_minutes"]),
        "| Verschil | " + fmt_signed_hm(data["total"]["diff_minutes"]),
        "| Norm tot nu toe | " + fmt_hm(data["norm_to_date_minutes"]),
        "| Geboekt tot nu toe | " + fmt_hm(data["booked_to_date_minutes"]),
        "| Verschil tot nu toe | " + fmt_signed_hm(data["diff_to_date_minutes"]),
        "| Gepland (na vandaag) | " + fmt_hm(data["planned_minutes"]),
        "| Dagen | " + str(data["total_days"]),
        "|===",
        "",
        f"=== Geboekte uren ten opzichte van de norm per maand in {data['year']}",
        "",
    ]
    headers = [
        "Maand", "Normdagen", "Norm", "Gewerkt", "waarvan reistijd",
        "waarvan declarabel", "Kilometers", "Ziek", "Vakantie", "Geboekt",
        "Verschil", "Geboekte dagen", "Niet-geboekte normdagen",
    ]
    rows = []
    for m in data["months"]:
        rows.append([
            MONTHS_NL[m["month"]],
            m["norm_days"],
            fmt_hm(m["norm_minutes"]),
            fmt_hm(m["worked_minutes"]),
            fmt_hm(m["travel_minutes"]),
            fmt_hm(m["travel_declarable_minutes"]),
            fmt_km(m["distance"]),
            fmt_hm(m["sick_minutes"]),
            fmt_hm(m["vacation_minutes"]),
            fmt_hm(m["booked_minutes"]),
            fmt_signed_hm(m["diff_minutes"]),
            m["booked_days"],
            m["unbooked_norm_days"],
        ])
    t = data["total"]
    rows.append([
        "Totaal",
        t["norm_days"],
        fmt_hm(t["norm_minutes"]),
        fmt_hm(t["worked_minutes"]),
        fmt_hm(t["travel_minutes"]),
        fmt_hm(t["travel_declarable_minutes"]),
        fmt_km(t["distance"]),
        fmt_hm(t["sick_minutes"]),
        fmt_hm(t["vacation_minutes"]),
        fmt_hm(t["booked_minutes"]),
        fmt_signed_hm(t["diff_minutes"]),
        data["total_days"],
        t["unbooked_norm_days"],
    ])
    lines.append(adoc_table(headers, rows))
    lines.append("")
    lines.append(
        f"Dagen (uniek, een dag met meerdere boekingen telt één keer): "
        f"*{data['total_days']}*."
    )
    past = t.get("past_unbooked_norm_days") or 0
    if past:
        lines += ["", f"WARNING: Verstreken normdagen zonder boeking: {past}."]
    holidays = data.get("holiday_weekdays") or []
    names = data.get("holiday_names") or {}
    lines += ["", "=== Feestdagen op werkdagen (niet in de norm)", ""]
    if not holidays:
        lines.append("Geen feestdagen op werkdagen.")
    else:
        for d in holidays:
            label = names.get(d) or ""
            lines.append(f"* `{d}`{f' — {label}' if label else ''}")
    lines.append("")
    return "\n".join(lines)


def render_checks(data):
    lines = ["== Controles", ""]
    counts = data.get("counts") or {}
    findings = data.get("findings") or []
    if not findings:
        lines.append("Geen bevindingen, alles ziet er goed uit.")
    else:
        lines += ["=== Bevindingen per type", ""]
        for ty in sorted(counts):
            if counts[ty]:
                lines.append(f"* {CHECK_TYPES.get(ty, ty)}: *{counts[ty]}*")
        lines += ["", f"=== Bevindingen: {len(findings)}", ""]
        rows = []
        for f in sorted(findings, key=lambda x: (x.get("date") or "", x.get("type") or "")):
            vs = ""
            if f.get("minutes") or f.get("norm_minutes"):
                vs = f"{fmt_hm(f.get('minutes') or 0)} / {fmt_hm(f.get('norm_minutes') or 0)}"
            rows.append([
                f.get("date") or "",
                CHECK_TYPES.get(f.get("type"), f.get("type")),
                "; ".join(f.get("labels") or []),
                vs,
            ])
        lines.append(adoc_table(["Datum", "Type", "Boekingen", "Geboekt / norm"], rows))
    lines += [
        "",
        f"Dagen (uniek): *{data.get('total_days', 0)}*.",
        "",
        "=== Omschrijvingsvarianten per project",
        "",
    ]
    variants = data.get("description_variants") or []
    if not variants:
        lines.append("Geen omschrijvingen gevonden.")
    else:
        for p in variants:
            cap = p.get("project") or ""
            if p.get("customer"):
                cap += f" ({p['customer']})"
            lines += ["", f"==== {adoc_escape(cap)}", ""]
            rows = [
                [v.get("description") or "(leeg)", v.get("count", 0), fmt_hm(v.get("minutes"))]
                for v in (p.get("variants") or [])
            ]
            lines.append(adoc_table(["Omschrijving", "Aantal", "Totale tijd"], rows))
    lines.append("")
    return "\n".join(lines)


def travel_acc_row(label, acc, days=None):
    return [
        label,
        acc.get("entries", 0),
        days if days is not None else acc.get("days", 0),
        fmt_hm(acc.get("total_minutes")),
        fmt_hm(acc.get("own_minutes")),
        fmt_hm(acc.get("declarable_minutes")),
        fmt_km(acc.get("distance")),
    ]


def render_travel(data):
    lines = ["== Reistijd", ""]
    missing = data.get("entries_without_distance") or 0
    if missing:
        lines += [f"WARNING: Reisboekingen zonder afstand: {missing}.", ""]
    cols = ["", "Boekingen", "Dagen", "Totale tijd", "Eigen tijd", "Declarabel", "Afstand (km)"]

    lines += ["=== Reistijd per richting", ""]
    rows = [
        travel_acc_row("Heen", data.get("out") or {}),
        travel_acc_row("Terug", data.get("back") or {}),
        travel_acc_row("Geen richting", data.get("no_direction") or {}),
        travel_acc_row("Totaal", data.get("total") or {}, days=data.get("total_days", 0)),
    ]
    lines.append(adoc_table(["Richting"] + cols[1:], rows))

    lines += ["", "=== Reistijd per maand", ""]
    rows = [travel_acc_row(MONTHS_NL[m["month"]], m) for m in data.get("months") or []]
    rows.append(travel_acc_row("Totaal", data.get("total") or {}, days=data.get("total_days", 0)))
    lines.append(adoc_table(["Maand"] + cols[1:], rows))

    lines += ["", "=== Reistijd per klant", ""]
    by_c = data.get("by_customer") or []
    if not by_c:
        lines.append("Geen reistijd geboekt.")
    else:
        rows = [
            travel_acc_row(r.get("customer") or "(geen klant)", r)
            for r in by_c
        ]
        lines.append(adoc_table(["Klant"] + cols[1:], rows))

    lines += ["", "=== Reistijd per route", ""]
    by_r = data.get("by_route") or []
    if not by_r:
        lines.append("Geen reistijd geboekt.")
    else:
        rows = []
        for r in by_r:
            rows.append([
                r.get("route") or "",
                r.get("customer") or "",
                r.get("entries", 0),
                r.get("days", 0),
                fmt_hm(r.get("total_minutes")),
                fmt_hm(r.get("own_minutes")),
                fmt_hm(r.get("declarable_minutes")),
                fmt_km(r.get("distance")),
            ])
        lines.append(adoc_table(
            ["Route", "Klant", "Boekingen", "Dagen", "Totale tijd", "Eigen tijd",
             "Declarabel", "Afstand (km)"],
            rows,
        ))
    lines += ["", f"Dagen (uniek): *{data.get('total_days', 0)}*.", ""]
    return "\n".join(lines)


def render_vacation(data):
    lines = [
        "== Vakantiesaldo",
        "",
        "[cols=\"2,1\"]",
        "|===",
        "| Vakantierecht | " + fmt_hm(data["entitlement_minutes"]),
        "| Overgedragen saldo | " + fmt_hm(data["carry_over_minutes"]),
        "| Beschikbaar | " + fmt_hm(data["available_minutes"]),
        "| Opgenomen | " + fmt_hm(data["used_minutes"]),
        "| Gepland | " + fmt_hm(data["planned_minutes"]),
        "| Resterend | " + fmt_hm(data["remaining_minutes"]),
        "| Resterend na gepland | " + fmt_hm(data["remaining_after_planned_minutes"]),
        "| Dagen | " + str(data["total_days"]),
        "|===",
        "",
    ]
    if not data.get("vacation_projects_set"):
        lines += [
            "WARNING: Er zijn geen vakantieprojecten ingesteld; vakantie wordt "
            "afgeleid van projectnamen.",
            "",
        ]
    lines += [f"=== Vakantie per maand in {data['year']}", ""]
    rows = [
        [MONTHS_NL[m["month"]], fmt_hm(m["minutes"]), m["days"]]
        for m in data.get("months") or []
    ]
    rows.append([
        "Totaal",
        fmt_hm((data.get("used_minutes") or 0) + (data.get("planned_minutes") or 0)),
        data.get("total_days", 0),
    ])
    lines.append(adoc_table(["Maand", "Vakantie", "Dagen"], rows))
    lines += ["", f"Dagen (uniek): *{data.get('total_days', 0)}*.", ""]
    return "\n".join(lines)


RENDERERS = {
    "norm": ("norm", render_norm),
    "checks": ("checks", render_checks),
    "travel": ("travel", render_travel),
    "vacation": ("vacation", render_vacation),
}


def main():
    args = parse_args()
    url = resolve_url(args.url)
    api_key = resolve_api_key(args.api_key, args.pass_path)
    headers = {"X-API-Key": api_key, "Accept": "application/json"}
    params = {"year": args.year}
    if args.user_id is not None:
        params["user_id"] = args.user_id

    me = api_get(url, "/auth/me", headers, None)
    who = me.get("display_name") or me.get("username") or "user"
    if args.user_id is not None:
        who = f"user_id={args.user_id}"

    sections = [s.strip().lower() for s in args.sections.split(",") if s.strip()]
    unknown = [s for s in sections if s not in RENDERERS]
    if unknown:
        raise SystemExit(f"Unknown section(s): {', '.join(unknown)}. "
                         f"Choose from: {', '.join(RENDERERS)}")

    parts = [
        f"= Time Tracking overzicht — {args.year}",
        f":author: {who}",
        f":revdate: {date.today().isoformat()}",
        ":pdf-page-size: A4",
        ":pdf-page-layout: landscape",
        ":toc:",
        ":toclevels: 2",
        ":sectnums!:",
        "",
        f"Gegenereerd vanuit WarmDesk (`{url}`) voor *{who}*, jaar *{args.year}*.",
        "",
    ]

    for name in sections:
        path, render = RENDERERS[name]
        data = api_get(url, f"/time-entries/overview/{path}", headers, params)
        parts.append(render(data))

    doc = "\n".join(parts).rstrip() + "\n"
    if args.out:
        Path(args.out).write_text(doc, encoding="utf-8")
        print(f"Wrote {args.out}", file=sys.stderr)
    else:
        sys.stdout.write(doc)


if __name__ == "__main__":
    main()
