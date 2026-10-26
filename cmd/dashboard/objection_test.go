package main

import (
	"strings"
	"testing"
)

func TestPrivacyHighlightsObjectionSeparatelyFromOtherRights(t *testing.T) {
	page := datenschutzPage(args{LegalEmail: "contact@example.test"})
	const start = `<section aria-labelledby="widerspruch">`
	_, rest, found := strings.Cut(page, start)
	if !found {
		t.Fatal("missing separate objection section")
	}
	section, _, found := strings.Cut(rest, "</section>")
	if !found {
		t.Fatal("objection section is not closed")
	}
	for _, want := range []string{
		`<h2 id="widerspruch">Widerspruchsrecht nach Art. 21 DSGVO</h2>`,
		`jederzeit`, `besonderen Situation`, `Art. 6 Abs. 1 lit. f DSGVO`, `Profiling`,
		`GitHub Pages und Cloudflare`, `personenbezogener Unternehmensprofile`, `Kontaktanfragen`,
		`zwingende schutzwürdige Gründe`, `Rechtsansprüchen`,
		`href="#datenschutz-kontakt"`,
	} {
		if !strings.Contains(section, want) {
			t.Errorf("missing objection information %q", want)
		}
	}
	for _, unrelated := range []string{"Einwilligung", "Art. 77", "Auskunft", "Medienprivileg"} {
		if strings.Contains(section, unrelated) {
			t.Errorf("objection notice mixed with other rights: %q", unrelated)
		}
	}
	for _, want := range []string{
		`id="datenschutz-kontakt"`,
		`<h2>Widerruf von Einwilligungen</h2>`,
		`Rechtmäßigkeit der Verarbeitung bis zum Widerruf bleibt unberührt`,
		`<h2>Betroffenenrechte</h2>`,
		`Art. 77 DSGVO`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing contact or separate right %q", want)
		}
	}
}
