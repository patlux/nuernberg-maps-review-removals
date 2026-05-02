#!/usr/bin/env python3
"""Fix categories in existing places.json using category_rules.json config."""

import json
import csv
import re
import sys
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


from typing import Optional

def clean_category(value: str, rules: dict) -> Optional[str]:
    """Replicates cleanCategoryCandidate logic using rules from JSON."""
    candidate = value.strip()
    if not candidate:
        return None

    # Strip Google Maps UI decorations
    candidate = re.sub(r"[·•]€?€?[·•]?", "", candidate)
    # Strip private-use Unicode (U+E000–U+F8FF) and box-drawing (U+2500–U+257F)
    candidate = "".join(
        c for c in candidate
        if not (0xE000 <= ord(c) <= 0xF8FF or 0x2500 <= ord(c) <= 0x257F)
    ).strip()

    while candidate and not (candidate[0].isalpha() or candidate[0].isdigit()):
        candidate = candidate[1:]
    while candidate and not (candidate[-1].isalpha() or candidate[-1].isdigit()):
        candidate = candidate[:-1]
    candidate = candidate.strip()

    if not candidate:
        return None

    lower = candidate.lower()

    # Reject URLs
    if re.search(r"(?i)\.(de|com|net|org|io|info)\b|https?://|www\.", candidate):
        return None

    # Reject too-long strings
    if len(candidate) > 50:
        return None

    # Reject review-like text (2+ review keyword hits)
    review_hits = sum(1 for w in rules["review_keywords"] if w in lower)
    if review_hits >= 2:
        return None

    # Reject business names containing |
    if "|" in candidate:
        return None

    # Reject exact blocked matches
    if lower in set(rules["blocked_strings"]):
        return None

    # Reject numbers, prices, postcodes, etc.
    if re.search(
        r"(?i)^[1-5](?:[,.][0-9])?$|^\(?[0-9][0-9.]*\)?$|€|geöffnet|geschlossen|adresse|telefon|\.de\b|\b9\d{4}\b",
        candidate,
    ):
        return None

    if not any(c.isalpha() for c in candidate):
        return None

    return candidate


def normalize(cat: str, rules: dict) -> str:
    """Normalize using the ordered category buckets from rules JSON."""
    lower = cat.lower()
    for bucket in rules["categories"]:
        if not bucket["keywords"]:  # catch-all (last bucket)
            return bucket["name"]
        for kw in bucket["keywords"]:
            if kw in lower:
                return bucket["name"]
    return rules["categories"][-1]["name"]


def fix_category(cat: Optional[str], rules: dict) -> Optional[str]:
    if cat is None or cat.strip() == "":
        return None
    # If the category is already a canonical bucket name, don't re-clean it.
    # The clean stage blocks raw Maps UI strings that happen to match bucket names
    # (e.g. "Lebensmittel" as a navigation chip vs "Lebensmittel" as our bucket).
    canonical_names = {b["name"].lower() for b in rules["categories"]}
    if cat.lower() in canonical_names:
        return cat
    cleaned = clean_category(cat, rules)
    if cleaned is None:
        return None
    return normalize(cleaned, rules)


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
    print(f"  {len(rules['categories'])} category buckets")
    print(f"  {len(rules['blocked_strings'])} blocked strings")
    print(f"  {len(rules['review_keywords'])} review keywords")
    print(f"  {len(rules['name_keywords'])} name keywords")
    print()

    print(f"Reading {PLACES_JSON} ...")
    with open(PLACES_JSON) as f:
        places = json.load(f)

    changed = 0
    removed = 0
    for place in places:
        old = place.get("category")
        new = fix_category(old, rules)
        if new is None and old is not None and place.get("status") == "success":
            # Try name-based inference as last resort
            name = place.get("name", "")
            if has_name_keyword(name, rules):
                new = normalize(name, rules)
        if old != new:
            changed += 1
            if new is None:
                removed += 1
            place["category"] = new

    print(f"Changed: {changed} categories ({removed} removed as garbage)")
    print(f"Total:   {len(places)} places")

    if changed == 0:
        print("Nothing to do.")
        return

    places.sort(key=lambda p: (p.get("postcode") or "", p.get("name") or ""))

    print(f"\nWriting {PLACES_JSON} ...")
    with open(PLACES_JSON, "w") as f:
        json.dump(places, f, indent="  ", ensure_ascii=False)
        f.write("\n")

    print(f"Writing {PLACES_CSV} ...")
    write_csv(places, PLACES_CSV)

    cats = Counter()
    nulls = 0
    for p in places:
        c = p.get("category")
        if c: cats[c] += 1
        else: nulls += 1

    print(f"\n=== Categories ({len(cats)} buckets, {nulls} null) ===")
    for cat, count in cats.most_common():
        print(f"  {count:5d}  {cat}")

    print("\nDone.")


if __name__ == "__main__":
    main()
