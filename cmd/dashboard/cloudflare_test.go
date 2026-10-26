package main

import (
	"strings"
	"testing"
)

func TestPrivacyDisclosesCloudflareDeliveryAndErrorReporting(t *testing.T) {
	page := datenschutzPage(args{})
	for _, want := range []string{
		`<h2>Cloudflare: Auslieferung und Sicherheit</h2>`,
		`Cloudflare, Inc., 101 Townsend St., San Francisco, CA 94107, USA`,
		`IP-Adresse`, `Art. 6 Abs. 1 lit. f DSGVO`,
		`Network Error Logging (NEL)`, `Report-To`, `a.nel.cloudflare.com`,
		`unabhängig von der optionalen Reichweitenanalyse`,
		`Nach eigener Angabe`, `EU-US Data Privacy Framework`, `Standardvertragsklauseln`,
		`https://www.cloudflare.com/privacypolicy/`,
		`https://developers.cloudflare.com/network-error-logging/`,
		`keine allgemeine Löschfrist für andere Cloudflare-Protokolle`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing Cloudflare disclosure %q", want)
		}
	}
	for _, unsupported := range []string{
		`Wir haben einen Auftragsverarbeitungsvertrag mit Cloudflare abgeschlossen`,
		`Cloudflare verarbeitet Daten ausschließlich in der EU`,
		`Cloudflare speichert keine IP-Adressen`,
	} {
		if strings.Contains(page, unsupported) {
			t.Errorf("unsupported assurance %q", unsupported)
		}
	}
}
