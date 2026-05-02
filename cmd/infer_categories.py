#!/usr/bin/env python3
"""Infer categories from place names for places without a category, using category_rules.json."""

import json
import csv
from pathlib import Path
from collections import Counter

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent
OUTPUT_DIR = PROJECT_ROOT / "output"
RULES_PATH = PROJECT_ROOT / "internal" / "mapsreview" / "data" / "category_rules.json"
PLACES_JSON = OUTPUT_DIR / "places.json"
PLACES_CSV = OUTPUT_DIR / "places.csv"


def load_rules():
    with open(RULES_PATH) as f:
        return json.load(f)


def normalize(cat: str, rules: dict) -> str:
    lower = cat.lower()
    for bucket in rules["categories"]:
        if not bucket["keywords"]:
            return bucket["name"]
        for kw in bucket["keywords"]:
            if kw in lower:
                return bucket["name"]
    return rules["categories"][-1]["name"]


def has_name_keyword(name: str, rules: dict) -> bool:
    lower = name.lower()
    return any(kw in lower for kw in rules["name_keywords"])


CSV_COLUMNS = [
    "id", "name", "postcode", "address", "rating", "reviewCount",
    "category", "lat", "lng", "bezirkId", "bezirkName",
    "hasDefamationNotice", "removedMin", "removedMax", "removedEstimate",
    "deletionRatioPct", "realRatingAdjusted", "removedText",
    "url", "readAt", "placeState", "status", "error",
]


def write_csv(rows: list[dict], path: Path):
    with open(path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(CSV_COLUMNS)
        for row in rows:
            w.writerow([
                row.get("id", ""), row.get("name", ""),
                row.get("postcode") or "", row.get("address") or "",
                str(row.get("rating") or ""), str(row.get("reviewCount") or ""),
                row.get("category") or "",
                str(row.get("lat") or ""), str(row.get("lng") or ""),
                row.get("bezirkId") or "", row.get("bezirkName") or "",
                str(row.get("hasDefamationNotice", False)).lower(),
                str(row.get("removedMin") or ""), str(row.get("removedMax") or ""),
                str(row.get("removedEstimate") or ""),
                str(row.get("deletionRatioPct") or ""),
                str(row.get("realRatingAdjusted") or ""),
                row.get("removedText") or "",
                row.get("url", ""), row.get("readAt", ""),
                row.get("placeState", ""), row.get("status", ""),
                row.get("error") or "",
            ])


def main():
    rules = load_rules()
    print(f"Loaded rules from {RULES_PATH}")

    with open(PLACES_JSON) as f:
        places = json.load(f)

    fixed = 0
    for p in places:
        if p.get("category") is None and p.get("status") == "success":
            name = p.get("name", "")
            if has_name_keyword(name, rules):
                p["category"] = normalize(name, rules)
                fixed += 1
                print(f"  {name[:55]:55s} → {p['category']}")

    print(f"\nInferred categories for {fixed} places")

    if fixed > 0:
        places.sort(key=lambda p: (p.get("postcode") or "", p.get("name") or ""))

        with open(PLACES_JSON, "w") as f:
            json.dump(places, f, indent="  ", ensure_ascii=False)
            f.write("\n")
        write_csv(places, PLACES_CSV)

        cats = Counter()
        nulls = 0
        for p in places:
            c = p.get("category")
            if c: cats[c] += 1
            else: nulls += 1
        print(f"Final: {len(cats)} buckets, {nulls} null")
        for cat, count in cats.most_common():
            print(f"  {count:5d}  {cat}")

    print("Done.")


if __name__ == "__main__":
    main()
