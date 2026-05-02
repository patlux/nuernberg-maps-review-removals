#!/usr/bin/env python3
"""Infer categories from place names for places without a category."""
import json
from pathlib import Path

OUTPUT_DIR = Path("output")
PLACES_JSON = OUTPUT_DIR / "places.json"
PLACES_CSV = OUTPUT_DIR / "places.csv"

# Category-indicating keywords (must appear in name to trigger inference)
NAME_KEYWORDS = [
    "café", "cafe", "caffè", "caffe", "kaffee", "coffee", "espresso",
    "bäckerei", "baeckerei", "konditorei", "backstube",
    "restaurant", "ristorante", "trattoria", "osteria",
    "pizzeria", "pizza",
    "burger",
    "sushi",
    "döner", "doener", "kebab",
    "imbiss", "grill", "hähnchen", "haehnchen", "chicken",
    "bar", "pub", "kneipe", "biergarten", "brauerei",
    "hotel", "pension", "gasthof", "gasthaus", "gästehaus", "gaestehaus",
    "bistro",
    "eis", "eisdiele", "eiscafé", "eiscafe",
    "steak",
]

# Same normalize function as fix_categories.py
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


def has_category_keyword(name: str) -> bool:
    lower = name.lower()
    return any(kw in lower for kw in NAME_KEYWORDS)


def main():
    with open(PLACES_JSON) as f:
        places = json.load(f)

    fixed = 0
    for p in places:
        if p.get("category") is None and p.get("status") == "success":
            name = p.get("name", "")
            if has_category_keyword(name):
                # Pass through normalize directly (name contains the keyword)
                p["category"] = normalize(name)
                fixed += 1
                print(f"  {name[:55]:55s} → {p['category']}")

    print(f"\nInferred categories for {fixed} places")

    if fixed > 0:
        # Sort
        places.sort(key=lambda p: (p.get("postcode") or "", p.get("name") or ""))

        with open(PLACES_JSON, "w") as f:
            json.dump(places, f, indent="  ", ensure_ascii=False)
            f.write("\n")

        # Regenerate CSV
        import csv
        CSV_COLS = [
            "id", "name", "postcode", "address", "rating", "reviewCount",
            "category", "lat", "lng", "bezirkId", "bezirkName",
            "hasDefamationNotice", "removedMin", "removedMax", "removedEstimate",
            "deletionRatioPct", "realRatingAdjusted", "removedText",
            "url", "readAt", "placeState", "status", "error",
        ]
        with open(PLACES_CSV, "w", newline="") as f:
            w = csv.writer(f)
            w.writerow(CSV_COLS)
            for p in places:
                w.writerow([
                    p.get("id", ""), p.get("name", ""),
                    p.get("postcode") or "", p.get("address") or "",
                    str(p.get("rating") or ""), str(p.get("reviewCount") or ""),
                    p.get("category") or "",
                    str(p.get("lat") or ""), str(p.get("lng") or ""),
                    p.get("bezirkId") or "", p.get("bezirkName") or "",
                    str(p.get("hasDefamationNotice", False)).lower(),
                    str(p.get("removedMin") or ""), str(p.get("removedMax") or ""),
                    str(p.get("removedEstimate") or ""),
                    str(p.get("deletionRatioPct") or ""),
                    str(p.get("realRatingAdjusted") or ""),
                    p.get("removedText") or "",
                    p.get("url", ""), p.get("readAt", ""),
                    p.get("placeState", ""), p.get("status", ""),
                    p.get("error") or "",
                ])

        # Final tally
        from collections import Counter
        cats = Counter()
        nulls = 0
        for p in places:
            c = p.get("category")
            if c:
                cats[c] += 1
            else:
                nulls += 1
        print(f"\nFinal: {len(cats)} categories, {nulls} null")
        for cat, count in cats.most_common():
            print(f"  {count:5d}  {cat}")

    print("\nDone.")


if __name__ == "__main__":
    main()
