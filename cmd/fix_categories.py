#!/usr/bin/env python3
"""Fix categories in existing places.json by applying cleanCategoryCandidate + normalizeCategory logic."""

import json
import csv
import re
import sys
from pathlib import Path

OUTPUT_DIR = Path("output")
PLACES_JSON = OUTPUT_DIR / "places.json"
PLACES_CSV = OUTPUT_DIR / "places.csv"

# ----- Blocked list (exact lowercased match → reject) -----
BLOCKED = {
    "restaurants in der nähe", "restaurants in der naehe",
    "verfügbarkeit prüfen", "verfuegbarkeit pruefen",
    "hotels", "mögliche aktivitäten", "moegliche aktivitaeten",
    "bars", "kaffee", "zum mitnehmen", "lebensmittel",
    "gespeichert", "zuletzt verwendet", "app herunterladen",
    "fotos ansehen", "übersicht", "speisekarte", "rezensionen",
    "info", "routenplaner", "speichern", "in der nähe",
    "in der naehe", "teilen", "bewertungen",
    "sortieren", "filtern", "alle anzeigen", "mehr anzeigen",
    "anrufen", "website", "route", "öffnet um", "oeffnet um",
    "geöffnet", "geschlossen", "heute geöffnet",
    "dauerhaft geschlossen", "vorübergehend geschlossen",
    "einkaufen", "dienstleistungen",
}

REVIEW_WORDS = [
    "schmeckt", "lecker", "sehr gut", "super", "top", "klasse",
    "komme", "wieder", "bedienung", "freundlich", "preise",
    "bestellt", "gegessen", "empfehlen", "enttäuscht",
    "der kaffee", "das essen", "die pizza", "der döner",
    "trinkgeld", "parkplatz", "bestellung", "lieferung",
    "zubereitet", "frisch", "atmosphäre", "gemütlich",
    "ich war", "ich bin", "ich kann", "wir haben", "wir waren",
]


from typing import Optional

def clean_category(value: str) -> Optional[str]:
    """Replicates cleanCategoryCandidate logic. Returns cleaned string or None if rejected."""
    candidate = value.strip()
    if not candidate:
        return None

    # Strip Google Maps UI decorations like "·Restaurant·" or "·€€·Restaurant"
    candidate = re.sub(r"[·•]€?€?[·•]?", "", candidate)

    # Strip private-use Unicode (U+E000–U+F8FF) and box-drawing (U+2500–U+257F)
    candidate = "".join(
        c for c in candidate
        if not (0xE000 <= ord(c) <= 0xF8FF or 0x2500 <= ord(c) <= 0x257F)
    ).strip()

    # Trim leading/trailing non-letter/number
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

    # Reject too-long strings (review snippets)
    if len(candidate) > 50:
        return None

    # Reject review-like text
    if sum(1 for w in REVIEW_WORDS if w in lower) >= 2:
        return None

    # Reject business names containing |
    if "|" in candidate:
        return None

    # Reject exact blocked matches
    if lower in BLOCKED:
        return None

    # Reject numbers, prices, postcodes, etc.
    if re.search(
        r"(?i)^[1-5](?:[,.][0-9])?$|^\(?[0-9][0-9.]*\)?$|€|geöffnet|geschlossen|adresse|telefon|\.de\b|\b9\d{4}\b",
        candidate,
    ):
        return None

    # Reject strings with no letters
    if not any(c.isalpha() for c in candidate):
        return None

    return candidate


# ----- Normalization to 12 canonical categories -----

def normalize(cat: str) -> str:
    lower = cat.lower()

    # 1. Café / Konditorei
    if any(kw in lower for kw in [
        "café", "cafe", "kaffee", "coffee", "espresso",
        "eisdiele", "eiscafé", "eiscafe", "ice cream",
        "teeladen", "tea", "bubble-tea", "bubble tea",
        "frühstück", "fruehstueck", "brunch",
        "konditorei", "patisserie", "confiserie",
        "tortenbäckerei", "tortenbaeckerei",
        "schokoladencafé", "schokoladencafe",
        "schokoladengeschäft", "schokoladengeschaeft",
        "süßwarengeschäft", "suesswarengeschaeft",
        "süßwaren", "suesswaren",
        "süßigkeiten", "suessigkeiten",
        "keksgeschäft", "keksgeschaeft",
        "donut-shop", "donut shop",
        "cafetek", "kindercafé", "kindercafe",
        "saftbar", "café mit frucht", "cafe mit frucht",
        "kunst-café", "kunst-cafe",
    ]):
        return "Café / Konditorei"

    # 2. Bäckerei
    if any(kw in lower for kw in [
        "bäckerei", "baeckerei", "backbedarf",
        "brezel", "bretzel",
        "großbäckerei", "grossbaeckerei",
        "hochzeitstortenbäckerei", "hochzeitstortenbaeckerei",
    ]):
        return "Bäckerei"

    # 3. Pizzeria
    if any(kw in lower for kw in [
        "pizzeria", "pizza-lieferdienst", "pizza lieferdienst", "pizza",
    ]):
        return "Pizzeria"

    # 4. Burger
    if "burger" in lower:
        return "Burger"

    # 5. Sushi
    if "sushi" in lower:
        return "Sushi"

    # 6. Döner / Kebab
    if any(kw in lower for kw in [
        "döner", "doener", "kebab",
        "türkisches restaurant", "tuerkisches restaurant",
    ]):
        return "Döner / Kebab"

    # 7. Asiatisch
    if any(kw in lower for kw in [
        "asiatisch", "chinesisch", "thailändisch", "thailaendisch",
        "vietnamesisch", "japanisch", "koreanisch",
        "indisch", "persisch", "ramen", "pho", "poke",
        "pan-asiatisch", "malaysisch", "hawaiianisch",
        "sri lanka", "afghanisch",
    ]):
        return "Asiatisch"

    # 8. Restaurant
    if any(kw in lower for kw in [
        "restaurant", "restaurants",
        "griechisch", "italienisch", "italienische",
        "deutsch", "fränkisch", "fraenkisch", "bayerisch",
        "mexikanisch", "spanisch",
        "mediterran", "orientalisch", "arabisch",
        "afrikanisch", "äthiopisch", "aethiopisch",
        "marokkanisch", "tunesisch", "ägyptisch", "aegyptisch",
        "libanesisch", "israelisch", "syrisch",
        "französisch", "franzoesisch",
        "österreichisch", "oesterreichisch", "tschechisch",
        "ukrainisch", "russisch", "polnisch",
        "rumänisch", "rumaenisch", "serbisch",
        "bulgarisch", "ungarisch",
        "argentinisch", "brasilianisch",
        "amerikanisch", "georgisch", "armenisch",
        "lateinamerikanisch", "tex-mex",
        "gourmetrestaurant", "gourmet",
        "steakhaus", "steak", "grillrestaurant", "grill",
        "fischrestaurant", "fisch", "meeresfrüchte", "meeresfruechte",
        "familienrestaurant", "familien",
        "mittagsrestaurant", "mittag",
        "halalrestaurant", "halal",
        "veganes restaurant", "vegan", "vegetarisch",
        "suppenrestaurant", "suppen",
        "tapasbar", "tapas", "buffet-restaurant", "buffet",
        "bratwurst", "diner",
        "food-court", "food court",
        "gaststätte", "gaststaette",
        "speiselokal", "gasthaus", "gasthof",
        "bistro",
    ]):
        return "Restaurant"

    # 9. Imbiss
    if any(kw in lower for kw in [
        "imbiss", "imbissrestaurant", "schnellimbiss",
        "fast-food", "fast food",
        "takeaway", "take-away", "take away",
        "zum mitnehmen",
        "lieferdienst", "lieferservice", "bringdienst",
        "catering",
        "sandwich", "falafel",
        "salat-shop", "salat shop",
        "hähnchen", "haehnchen", "chicken",
        "kantine",
    ]):
        return "Imbiss"

    # 10. Bar
    if any(kw in lower for kw in [
        "bar", "bars",
        "biergarten", "bier",
        "kneipe", "gastrokneipe",
        "pub", "shisha",
        "cocktail", "lounge",
        "weinstube", "weinlokal",
        "stehbar",
        "brauerei", "brauereischänke", "brauereischaenke",
        "nachtclub",
    ]):
        return "Bar"

    # 11. Lebensmittel
    if any(kw in lower for kw in [
        "supermarkt", "discounter",
        "lebensmittel", "bioladen",
        "feinkost", "delikatessen",
        "fleischerei", "metzgerei",
        "fischgeschäft", "fischgeschaeft",
        "obst", "gemüse", "gemuese",
        "getränke", "getraenke", "weinhandlung",
        "kiosk",
    ]):
        return "Lebensmittel"

    # 12. Sonstiges
    return "Sonstiges"


def fix_category(cat: Optional[str]) -> Optional[str]:
    if cat is None or cat.strip() == "":
        return None
    cleaned = clean_category(cat)
    if cleaned is None:
        return None  # removed as garbage
    return normalize(cleaned)


# ----- Write CSV -----

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
                row.get("id", ""),
                row.get("name", ""),
                row.get("postcode") or "",
                row.get("address") or "",
                _float_str(row.get("rating")),
                _int_str(row.get("reviewCount")),
                row.get("category") or "",
                _float_str(row.get("lat")),
                _float_str(row.get("lng")),
                row.get("bezirkId") or "",
                row.get("bezirkName") or "",
                str(row.get("hasDefamationNotice", False)).lower(),
                _int_str(row.get("removedMin")),
                _int_str(row.get("removedMax")),
                _float_str(row.get("removedEstimate")),
                _float_str(row.get("deletionRatioPct")),
                _float_str(row.get("realRatingAdjusted")),
                row.get("removedText") or "",
                row.get("url", ""),
                row.get("readAt", ""),
                row.get("placeState", ""),
                row.get("status", ""),
                row.get("error") or "",
            ])


def _float_str(v) -> str:
    if v is None:
        return ""
    return str(v)


def _int_str(v) -> str:
    if v is None:
        return ""
    return str(v)


# ----- Main -----

def main():
    print(f"Reading {PLACES_JSON} ...")
    with open(PLACES_JSON) as f:
        places = json.load(f)

    changed = 0
    removed = 0
    for place in places:
        old = place.get("category")
        new = fix_category(old)
        if old != new:
            changed += 1
            if new is None:
                removed += 1
            place["category"] = new

    print(f"Changed: {changed} categories ({removed} removed as garbage)")
    print(f"Total:   {len(places)} places")

    # Sort (postcode then name)
    places.sort(key=lambda p: (p.get("postcode") or "", p.get("name") or ""))

    print(f"\nWriting {PLACES_JSON} ...")
    with open(PLACES_JSON, "w") as f:
        json.dump(places, f, indent="  ", ensure_ascii=False)
        f.write("\n")

    print(f"Writing {PLACES_CSV} ...")
    write_csv(places, PLACES_CSV)

    # Show new distribution
    from collections import Counter
    cats = Counter()
    for p in places:
        c = p.get("category")
        cats[c if c else "(leer)"] += 1

    print(f"\n=== Neue Kategorien ({len(cats)}) ===")
    for cat, count in cats.most_common():
        print(f"  {count:5d}  {cat}")

    print("\nDone.")


if __name__ == "__main__":
    main()
