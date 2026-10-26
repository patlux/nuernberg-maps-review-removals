package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var legalPageFiles = []string{"methodik.html", "korrektur.html", "impressum.html", "datenschutz.html"}

func legalPagesEnabled(a args) bool {
	return strings.TrimSpace(a.LegalName) != "" || strings.TrimSpace(a.LegalEmail) != "" || len(a.LegalAddressLines) > 0 || strings.TrimSpace(a.LegalNote) != "" || strings.TrimSpace(a.LegalPostHandler) != ""
}

func validateLegalArgs(a args) error {
	if !legalPagesEnabled(a) {
		return nil
	}
	missing := []string{}
	if strings.TrimSpace(a.LegalName) == "" {
		missing = append(missing, "--legal-name")
	}
	if strings.TrimSpace(a.LegalEmail) == "" {
		missing = append(missing, "--legal-email")
	}
	if len(a.LegalAddressLines) == 0 {
		missing = append(missing, "--legal-address-line")
	}
	if len(missing) > 0 {
		return fmt.Errorf("legal pages require %s", strings.Join(missing, ", "))
	}
	return nil
}

func syncLegalPages(a args, outputDir string) error {
	if !legalPagesEnabled(a) {
		for _, name := range legalPageFiles {
			if err := os.Remove(filepath.Join(outputDir, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	pages := map[string]string{
		"methodik.html":    methodikPage(a),
		"korrektur.html":   korrekturPage(a),
		"impressum.html":   impressumPage(a),
		"datenschutz.html": datenschutzPage(a),
	}
	for name, body := range pages {
		if err := os.WriteFile(filepath.Join(outputDir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func legalNavHTML(a args) string {
	if !legalPagesEnabled(a) {
		return ""
	}
	return `<nav aria-label="Weiterführende Hinweise"><a href="methodik.html">Methodik</a><a href="korrektur.html">Korrektur melden</a><a href="impressum.html">Impressum</a><a href="datenschutz.html">Datenschutz</a></nav>`
}

func footerLegalLinksHTML(a args) string {
	if !legalPagesEnabled(a) {
		return ""
	}
	return `<div class="footer-privacy"><a href="methodik.html">Methodik</a> · <a href="korrektur.html">Korrektur melden</a> · <a href="impressum.html">Impressum</a> · <a href="datenschutz.html">Datenschutz</a></div>`
}

func cartoTilesEnabled(a args) bool {
	return a.CartoEnabled && a.CartoPrivacyText != "" && legalPagesEnabled(a)
}

func mapConsentHTML(a args) string {
	text := "Die Karte zeigt die eingebetteten Orte und Bezirksflächen. Leaflet wird mit dieser Website ausgeliefert. Externe Kartenkacheln sind deaktiviert; beim Aktivieren werden keine Kartenanbieter kontaktiert."
	if cartoTilesEnabled(a) {
		text = "Mit dem Laden externer CARTO-Kartenkacheln werden IP-Adresse, Browserinformationen und der angefragte Kartenausschnitt übertragen. Weitere Abrufe erfolgen beim Verschieben, Zoomen und Designwechsel. " + a.CartoPrivacyText
	}
	link := ""
	if legalPagesEnabled(a) {
		link = ` <a href="datenschutz.html">Datenschutzdetails</a>.`
	}
	return `<div class="map-consent"><div class="map-consent-inner"><p class="map-consent-text">` + esc(text) + `</p><button id="mapConsentLoad" type="button" class="map-consent-btn">Karte laden</button><p class="map-consent-link">Die Karte kann jederzeit mit „Karte deaktivieren“ geschlossen werden.` + link + `</p></div></div>`
}

func legalAddressHTML(a args) string {
	lines := []string{esc(a.LegalName)}
	for _, line := range a.LegalAddressLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, esc(line))
		}
	}
	return strings.Join(lines, "<br>")
}

func legalShell(title string, description string, body string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="de">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s · Nürnberg Google-Maps-Bewertungen</title>
  <meta name="description" content="%s">
  <style>body{margin:0;font-family:Arial,Helvetica,sans-serif;color:#333;background:#fff}main{width:min(920px,calc(100vw - 32px));margin:42px auto 70px;line-height:1.6}nav{display:flex;flex-wrap:wrap;gap:12px;margin-bottom:30px}a{color:#cf2a1b;font-weight:700;text-decoration:none}a:hover{text-decoration:underline}h1{font-size:clamp(30px,5vw,48px);line-height:1.1;margin:0 0 18px}h2{margin-top:30px;color:#222}.note,.card{padding:16px;border-left:5px solid #1f6f8b;background:#f6f6f6}@media(prefers-color-scheme:dark){body{color:#e7e2dc;background:#0e0c0b}h2{color:#fff5ec}.note,.card{background:#171412}}</style>
</head>
<body><main>
  <nav aria-label="Navigation"><a href="/">Dashboard</a><a href="methodik.html">Methodik</a><a href="korrektur.html">Korrektur melden</a><a href="impressum.html">Impressum</a><a href="datenschutz.html">Datenschutz</a></nav>
%s
</main></body>
</html>
`, esc(title), escAttr(description), body)
}

func methodikPage(a args) string {
	body := fmt.Sprintf(`<h1>Methodik und Einordnung</h1>
 <p class="note">Historische Momentaufnahme öffentlich sichtbarer Google-Hinweise. Ein Hinweis belegt weder rechtswidriges Verhalten noch missbräuchliche Beschwerden oder die Unterdrückung berechtigter Kritik.</p>
 <h2>Was erfasst wird</h2>
 <p>Das Dashboard dokumentiert Google-Maps-Profile im Raum %s. Erfasst werden Profilname, Kategorie, Adresse, Koordinaten, Google-Link, angezeigte Sternebewertung und Rezensionszahl, Hinweisspanne und Abrufzeitpunkt. Die Website veröffentlicht keine Rezensionstexte und keine Namen von Bewertenden oder Beschwerdeführenden.</p>
 <p>Google beschreibt die Hinweise als Entfernungen wegen Diffamierungsbeschwerden innerhalb der letzten 365 Tage. Dieser Zeitraum bezieht sich auf den damaligen Abruf, nicht auf den heutigen Besuch dieser Website. Google nennt Bereiche wie 21–50 oder über 250, keine durchgehend exakten Zahlen.</p>
 <h2>Erhebung und Datenstand</h2>
 <p>Profile werden über Google-Maps-Suchen mit Kategoriebegriffen und Postleitzahlen gefunden und automatisiert ausgelesen. Optional dient die Places API der Ortssuche. Das ist keine vollständige Erhebung aller Betriebe und keine manuelle Einzelprüfung sämtlicher Ergebnisse.</p>
 <p>Der Zeitpunkt unter „Datenabruf“ gehört zum jeweiligen gespeicherten Eintrag. Der neueste Abruf im Dashboard ist kein gemeinsamer Prüfzeitpunkt aller Profile. Fehlgeschlagene Abrufe oder später nicht mehr erkannte Hinweise können zur Beibehaltung eines älteren Datensatzes mit dessen ursprünglichem Abrufdatum führen. Ein unveränderter Eintrag bedeutet deshalb nicht, dass der Hinweis heute noch sichtbar ist.</p>
 <p>Beim nächsten Abruf nicht mehr erkannte Hinweise dürfen nicht allein als bestätigte Löschung des Hinweises behandelt werden. Sie benötigen eine Nachprüfung. Bis dahin beschreibt das Dashboard ausschließlich die ältere Beobachtung, keine aktuelle Bestätigung.</p>
 <h2>Spannen und Modellanteile</h2>
 <p>Die Übersicht sortiert nach der beobachteten Google-Spanne, innerhalb derselben Spanne alphabetisch. Aus der Spanne wird keine Bewertung der Betriebsqualität abgeleitet.</p>
 <p>Für zusätzliche Modellkennzahlen verwendet das Dashboard den Mittelpunkt geschlossener Spannen, zum Beispiel 35,5 für 21–50. Für über 250 wird der frei gewählte Rechenwert 300 verwendet; er ist keine Schätzung mit statistisch belegter Genauigkeit, und die tatsächliche Anzahl kann höher oder niedriger sein.</p>
 <p>Der Modellanteil wird als <code>Näherungswert / (sichtbare Rezensionen + Näherungswert)</code> berechnet. Die sichtbare Rezensionszahl ist ein Bestand zum Abrufzeitpunkt, die Hinweisspanne betrifft einen Zeitraum. Der Quotient ist daher keine gemessene jährliche Löschquote. Die Gesamtzahl der Rezensionen im selben Zeitraum ist nicht bekannt.</p>
 <h2>1-Stern-Szenario</h2>
 <p>Diese Was-wäre-wenn-Rechnung ergänzt die sichtbaren Rezensionen rechnerisch um den oben beschriebenen Näherungswert entfernter Rezensionen und setzt für jede davon 1 Stern an. Sie wird als <code>(Google-Rating × sichtbare Rezensionen + Näherungswert × 1) / (sichtbare Rezensionen + Näherungswert)</code> berechnet. Das bereits gerundete Google-Rating wird als Ausgangswert verwendet; zwei angezeigte Nachkommastellen sind keine Aussage über die Genauigkeit.</p>
 <p>Das Ergebnis ist kein tatsächliches oder „wahres“ Rating und keine gesicherte Untergrenze. Anzahl und Sterne der entfernten Rezensionen sind unbekannt. Es gibt insbesondere keinen Nachweis, dass diese Rezensionen alle 1 Stern hatten. Auch ist die Hinweisspanne auf einen Zeitraum bezogen, während das angezeigte Rating und die sichtbare Rezensionszahl einen Bestand abbilden. Das Szenario rekonstruiert deshalb kein historisches Rating.</p>
 <p>Beispiel: Bei 4,5 Sternen, 100 sichtbaren Rezensionen und der Spanne 21–50 ergibt der Mittelpunkt 35,5 einen Szenariowert von rund 3,58. Mit 50 angenommenen 1-Stern-Rezensionen wären es rund 3,33. Der Mittelpunkt liefert also keinen gesicherten niedrigsten Wert. Für „über 250“ wird nur der frei gewählte Rechenwert 300 eingesetzt; eine endliche Obergrenze ist nicht bekannt.</p>
 <p>Die Szenarioansicht zeigt nur Profile mit beobachtetem Hinweis und gültigen Rechenwerten, standardmäßig nach aufsteigendem Szenariowert. Das ist keine Bewertung der Betriebsqualität oder der Berechtigung von Beschwerden. Fehlende Werte werden nicht als 0 Sterne dargestellt.</p>
 <h2>Grenzen und personenbezogene Profile</h2>
 <p>Kein Google-Hinweis beim Abruf bedeutet nicht, dass keine Rezensionen entfernt wurden. Ein vorhandener Hinweis sagt nichts darüber aus, ob eine Beschwerde berechtigt war. Sprache, Oberfläche, Suchauswahl und technische Erkennung können die Ergebnisse beeinflussen. Die Stichprobe umfasst auch Orte im Umland und ist nicht repräsentativ für alle Betriebe.</p>
 <p>Heilberufe und personenbezogene Einzelpraxen können durch eine Namensnennung besonders unmittelbar betroffen sein. Das Dashboard trifft keine Aussage über fachliche Qualität, Behandlungsstandards oder Berufsausübung. Konkrete Fehler, veraltete Zuordnungen und rechtliche Beanstandungen können über die Korrekturseite gemeldet werden.</p>
 <h2>Veröffentlichung und Nachvollziehbarkeit</h2>
 <p>Die Website enthält die für Filter, Tabelle und Karte erforderlichen Profildaten direkt im HTML; sie sind technisch auslesbar. Es werden keine zusätzlichen Rohdaten-CSV, separaten Ranglisten oder alten Diagrammdateien in das Website-Artefakt übernommen. Diese Beschränkung macht die im Dashboard angezeigten Daten nicht geheim und betrifft nicht etwaige gesonderte Quellcode-Repositories.</p>
 <p>Die Google-Profile sind verlinkt. Ihr heutiger Inhalt kann von der historischen Beobachtung abweichen. Lokale technische Fehlerdiagnosen können vollständige Seitenabbilder enthalten; sie gehören nicht zur Website-Veröffentlichung und müssen gesondert auf Datenminimierung und Aufbewahrung geprüft werden.</p>
 <h2>Journalistische Sorgfalt (§ 19 MStV)</h2>
 <p>§ 19 Abs. 1 Satz 3 MStV verlangt im jeweiligen Anwendungsbereich die Prüfung von Nachrichten auf Inhalt, Herkunft und Wahrheit mit der nach den Umständen gebotenen Sorgfalt. Automatische Erkennung ersetzt keine erforderliche Nachprüfung. Fehlerhinweise werden über die Korrekturseite entgegengenommen; bestätigte Fehler sollen berichtigt und rechtlich unzulässige Inhalte entfernt werden.</p>
 <p><a href="korrektur.html">Korrektur melden</a> · <a href="/">Zurück zum Dashboard</a></p>`, esc(a.City))
	return legalShell("Methodik und Einordnung", "Historischer Datenstand, Berechnungen und Grenzen des Google-Hinweis-Dashboards.", body)
}

func korrekturPage(a args) string {
	return legalShell("Korrektur melden", "Fehler, veraltete Daten und rechtliche Beanstandungen melden.", fmt.Sprintf(`<h1>Korrektur melden</h1>
 <p class="note">Hinweis fehlerhaft, veraltet oder missverständlich? Bitte sende den betroffenen Eintrag oder Google-Profil-Link und eine kurze Begründung an <!--email_off--><a href="mailto:%s">%s</a><!--/email_off-->.</p>
 <h2>Prüfung und vorläufige Ausblendung</h2>
 <p>Geprüft werden konkrete Fehler, Verwechslungen, veraltete Zuordnungen und rechtliche Beanstandungen. Bitte nenne nach Möglichkeit den angezeigten Abrufzeitpunkt. Ein inzwischen fehlender Google-Hinweis kann für die Einordnung wichtig sein, widerlegt aber nicht automatisch eine frühere Beobachtung.</p>
 <p>Bei plausiblen Beanstandungen kann ein Eintrag bis zur Klärung vorläufig ausgeblendet werden. Das bedeutet kein Anerkenntnis einer Rechtsverletzung. Nach Prüfung wird berichtigt, entfernt oder nachvollziehbar wieder angezeigt. Gesetzliche Ansprüche bleiben unberührt.</p>
 <h2>Gegendarstellungsanspruch</h2>
 <p>Soweit die gesetzlichen Voraussetzungen vorliegen, richtet sich eine Gegendarstellung bei journalistisch-redaktionellen Telemedien nach <a href="https://www.gesetze-bayern.de/Content/Document/MStV-20" target="_blank" rel="noopener noreferrer">§ 20 MStV</a>. Die Vorschrift regelt insbesondere Form, Fristen sowie Art und Dauer der Veröffentlichung. Diese Seite ersetzt keine Prüfung des Einzelfalls.</p>
 <p>§ 20 Abs. 2 Nr. 4 MStV verlangt einen unverzüglichen schriftlichen Zugang mit Unterschrift des Betroffenen oder seines gesetzlichen Vertreters, spätestens sechs Wochen nach dem letzten Angebotstag des beanstandeten Textes, jedenfalls jedoch drei Monate nach erstmaliger Einstellung. Formelle Verlangen sind an die Anschrift im Impressum zu richten. Für einen einfachen Fehlerhinweis genügt eine E-Mail; dafür wird keine eigenhändige Unterschrift verlangt.</p>
 <p>Eine gesetzlich geschuldete Gegendarstellung wird nach Maßgabe des § 20 MStV veröffentlicht, insbesondere ohne Einschaltungen oder Weglassungen, in gleicher Aufmachung und mit der vorgeschriebenen Verknüpfung und Dauer.</p>
 <p><a href="impressum.html">Impressum und Anschrift</a> · <a href="/">Zurück zum Dashboard</a></p>`, escAttr(a.LegalEmail), esc(a.LegalEmail)))
}

func impressumPage(a args) string {
	note := ""
	if strings.TrimSpace(a.LegalNote) != "" {
		note = fmt.Sprintf("\n    <p>%s</p>", esc(a.LegalNote))
	}
	body := fmt.Sprintf(`<h1>Impressum</h1>
  <section class="card" aria-label="Verantwortlich">
    <h2>Angaben gemäß § 5 DDG</h2>
    <p><strong>%s</strong><br>%s</p>
    <p>E-Mail: <!--email_off--><a href="mailto:%s">%s</a><!--/email_off--></p>%s
    <h2>Verantwortlich für journalistisch-redaktionelle Inhalte</h2>
    <p>Verantwortlich im Sinne von § 18 Abs. 2 MStV:<br>%s</p>
  </section>
  <h2>Hinweise zu den Inhalten</h2>
  <p>Das Angebot dokumentiert historische Beobachtungen. Datenstand und Grenzen stehen in der Methodik. Fehler und rechtliche Beanstandungen können über die Korrekturseite gemeldet werden. Gesetzliche Verantwortlichkeiten bleiben unberührt.</p>
  <p><a href="/">Zurück zum Dashboard</a></p>`, esc(a.LegalName), strings.Join(legalAddressLinesHTML(a), "<br>"), escAttr(a.LegalEmail), esc(a.LegalEmail), note, legalAddressHTML(a))
	return legalShell("Impressum", "Impressum des Dashboards zu Google-Maps-Bewertungen.", body)
}

func legalAddressLinesHTML(a args) []string {
	out := []string{}
	for _, line := range a.LegalAddressLines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, esc(line))
		}
	}
	return out
}

func datenschutzPage(a args) string {
	analytics := "<p>In dieser Ausgabe ist keine Reichweitenanalyse aktiviert. Es wird kein Plausible-Skript geladen.</p>"
	if plausibleAnalyticsSrc(a) != "" {
		analytics = "<p>Nach Einwilligung kann Plausible geladen werden. Dabei werden zunächst einzelne Nutzungsereignisse übertragen; aggregierte Berichte bedeuten nicht, dass nur aggregierte Daten verarbeitet werden. Rechtsgrundlage ist Art. 6 Abs. 1 lit. a DSGVO. Die Einwilligung wird unter dashboardAnalyticsConsent im Sitzungsspeicher dieses Tabs festgehalten; Grundlage für diese gewünschte Einstellung ist § 25 Abs. 2 Nr. 2 TDDDG. Sie kann über die Analytics-Einstellung im Footer widerrufen werden.</p><p>" + esc(a.AnalyticsPrivacyText) + "</p>"
	}
	maps := "<p>Leaflet wird zusammen mit dieser Website ausgeliefert. Die Karte verwendet eingebettete Orts- und Bezirksdaten. Externe Kartenkacheln sind in dieser Ausgabe deaktiviert. Beim Aktivieren der Karte werden keine externen Kartenanbieter kontaktiert.</p>"
	if cartoTilesEnabled(a) {
		maps = "<p>Erst nach Klick auf „Karte laden“ werden externe CARTO-Kacheln angefordert. Dabei werden IP-Adresse, Browserinformationen und der Kartenausschnitt übertragen. Beim Verschieben, Zoomen oder Designwechsel entstehen weitere Abrufe. Rechtsgrundlage für die optionale Einbindung ist Art. 6 Abs. 1 lit. a DSGVO. „Karte deaktivieren“ beendet künftige Kachelabrufe; bereits übermittelte Daten werden dadurch nicht zurückgerufen.</p><p>" + esc(a.CartoPrivacyText) + "</p>"
	}
	note := ""
	if a.LegalNote != "" {
		note = "<p>" + esc(a.LegalNote) + "</p>"
	}
	post := ""
	if a.LegalPostHandler != "" {
		post = "<h2>Zustellanschrift</h2><p>" + esc(a.LegalPostHandler) + " kann Post für den Verantwortlichen entgegennehmen und weiterleiten. Dafür werden die erforderlichen Absender-, Zustell- und Inhaltsdaten verarbeitet. Rechtsgrundlage ist Art. 6 Abs. 1 lit. f DSGVO; Interesse ist die zuverlässige Erreichbarkeit.</p>"
	}
	body := fmt.Sprintf(`<h1>Datenschutzerklärung</h1>
 <h2>Verantwortlicher</h2><p>%s<br>E-Mail: <!--email_off--><a href="mailto:%s">%s</a><!--/email_off--></p>%s
 <h2>Bereitstellung der Website</h2>
 <p>Die Website wird über GitHub Pages bereitgestellt. GitHub, Inc., USA, verarbeitet für Auslieferung und Sicherheit insbesondere IP-Adresse, Zeitpunkt, angeforderte Datei, technische Statusdaten sowie gegebenenfalls Referrer und Browserinformationen. Rechtsgrundlage ist Art. 6 Abs. 1 lit. f DSGVO; berechtigtes Interesse ist die sichere Bereitstellung des Informationsangebots.</p>
 <p>Weitere Angaben zu GitHubs Verarbeitung, Empfängern, Aufbewahrung und internationalen Übermittlungen stehen in der <a href="https://docs.github.com/en/site-policy/privacy-policies/github-general-privacy-statement" target="_blank" rel="noopener noreferrer">GitHub-Datenschutzerklärung</a>. Eine Übermittlung in die USA setzt eine anwendbare Grundlage nach Kapitel V DSGVO voraus; eine Berufung auf das EU-US Data Privacy Framework setzt eine gültige, die Verarbeitung erfassende Zertifizierung voraus. Diese Website behauptet keine eigene Prüfung sämtlicher Hostingverträge oder konkreter Log-Löschfristen.</p>
 <h2>Cloudflare: Auslieferung und Sicherheit</h2>
 <p>Der Website ist Cloudflare, Inc., 101 Townsend St., San Francisco, CA 94107, USA, als Reverse-Proxy und Content Delivery Network vorgeschaltet. Anfragen erreichen zunächst Cloudflare und werden dort ausgeliefert oder an GitHub Pages weitergeleitet. Cloudflare verarbeitet dabei insbesondere IP-Adresse, angeforderte URL, Zeitpunkt, Browserinformationen sowie Verbindungs- und Sicherheitsdaten. Zweck sind die zuverlässige Auslieferung, TLS-Verschlüsselung und der Schutz vor Angriffen, insbesondere DDoS. Rechtsgrundlage ist Art. 6 Abs. 1 lit. f DSGVO; berechtigtes Interesse ist die sichere und verfügbare Bereitstellung dieser Website.</p>
 <p>Cloudflare setzt außerdem die Antwort-Header <code>NEL</code> und <code>Report-To</code> für Network Error Logging (NEL). Unterstützende Browser können dadurch Netzwerkfehlerberichte an <code>a.nel.cloudflare.com</code> senden. Diese können angefragte URLs, Referrer, Fehlerart, Status und Verbindungsdauer enthalten; beim Berichtsempfang wird auch die IP-Adresse verarbeitet. Dies dient der Erkennung und Diagnose von Verbindungsproblemen und erfolgt unabhängig von der optionalen Reichweitenanalyse. Rechtsgrundlage für diese Verarbeitung ist ebenfalls Art. 6 Abs. 1 lit. f DSGVO; berechtigtes Interesse ist die Erkennung und Behebung von Auslieferungsfehlern.</p>
 <p>Eine Verarbeitung außerhalb der EU, insbesondere in den USA, ist möglich. Nach eigener Angabe stützt Cloudflare Übermittlungen aus dem EWR in die USA auf seine Zertifizierung zum EU-US Data Privacy Framework; für andere internationale Übermittlungen sowie bei Wegfall dieser Grundlage nennt Cloudflare Standardvertragsklauseln und gegebenenfalls ergänzende Maßnahmen. Einzelheiten zu Empfängern, Übermittlungen und Aufbewahrung stehen in der <a href="https://www.cloudflare.com/privacypolicy/" target="_blank" rel="noopener noreferrer">Cloudflare-Datenschutzerklärung</a>. Daraus folgt keine Bestätigung einer gesonderten Prüfung unserer konkreten Vertrags- und Kontoeinstellungen.</p>
 <p>Laut <a href="https://developers.cloudflare.com/network-error-logging/" target="_blank" rel="noopener noreferrer">Cloudflares NEL-Dokumentation</a> wird die Client-IP für NEL nur während der Verarbeitung des Berichts im flüchtigen Speicher gehalten und danach verworfen; daraus werden unter anderem Netzbetreiber und ungefähre Region abgeleitet. Diese Angabe betrifft nur die NEL-Verarbeitung und ist keine allgemeine Löschfrist für andere Cloudflare-Protokolle. Für weitere Daten nennt Cloudflare als Aufbewahrungskriterien insbesondere den jeweiligen Verarbeitungszweck und gesetzliche Pflichten.</p>
 <h2>Reichweitenanalyse</h2>%s
 <h2>Lokale Darstellungseinstellung</h2>
 <p>Erst nach Betätigung des Hell-/Dunkel-Schalters wird die Auswahl unter <code>dashboardTheme</code> im lokalen Browserspeicher abgelegt. Sie wird nicht an den Betreiber gesendet und bleibt bis zum Löschen der Website-Daten im Browser gespeichert. Grundlage für die gewünschte Darstellungsfunktion ist § 25 Abs. 2 Nr. 2 TDDDG.</p>
 <h2>Karte</h2>%s
 <h2>Standortfunktion</h2>
 <p>„In meiner Nähe“ fordert den Standort nur nach aktiver Auswahl und Freigabe durch den Browser an. Das Dashboard berechnet Entfernungen lokal und speichert die Koordinaten nicht dauerhaft. Rechtsgrundlage ist Art. 6 Abs. 1 lit. a DSGVO. Bei aktivierten externen Kartenkacheln kann der angefragte Kartenausschnitt Rückschlüsse auf den ungefähren Standort erlauben. Die Standortfreigabe kann in den Browser-Einstellungen widerrufen werden; Neuladen entfernt die im Dashboard gehaltenen Koordinaten.</p>
 <h2>Externe Links</h2>
 <p>Google Maps und andere verlinkte Angebote werden erst beim Anklicken geöffnet. Dann verarbeiten die jeweiligen Anbieter die beim Seitenaufruf anfallenden Daten nach ihren eigenen Datenschutzbedingungen.</p>
 <h2>Öffentliche Unternehmensprofile</h2>
 <p>Quelle ist Google Maps. Veröffentlicht werden Profilname, Kategorie, Adresse, Koordinaten, Google-Link, angezeigte Bewertung und Rezensionszahl, beobachtete Hinweisspanne, daraus abgeleitete Modellanteile, das hypothetische 1-Stern-Szenario und Abrufzeitpunkt. Bei Einzelunternehmen und personenbezogenen Profilen können dies personenbezogene Daten sein. Die im HTML enthaltenen Daten sind für alle Besucher auslesbar; es werden keine Rezensionstexte oder Bewertendenprofile veröffentlicht.</p>
 <p>Zweck ist die historische Dokumentation und Einordnung öffentlich sichtbarer Google-Hinweise. Soweit die allgemeinen DSGVO-Regeln gelten, ist die Rechtsgrundlage Art. 6 Abs. 1 lit. f DSGVO. Das Informationsinteresse ist gegen die Rechte der Betroffenen abzuwägen. Der Datenbestand bleibt für die historische Dokumentation verfügbar, soweit dies erforderlich und rechtmäßig ist; konkrete Fehler, veraltete Zuordnungen und Widersprüche sind Anlass einer erneuten Prüfung.</p>
 <p>Informationen nach Art. 14 DSGVO betreffen auch Daten, die nicht direkt bei der betroffenen Person erhoben wurden. Diese öffentliche Erklärung ersetzt eine im Einzelfall erforderliche individuelle Information nicht automatisch. Ausnahmen und ihre Voraussetzungen müssen gesondert geprüft werden.</p>
 %s
 <h2>Kontakt und Korrekturanfragen</h2>
 <p>Per E-Mail übermittelte Kontaktdaten, Nachrichten und Profil-Links werden zur Bearbeitung und Dokumentation der Anfrage verarbeitet. Grundlage ist Art. 6 Abs. 1 lit. f DSGVO, bei Erfüllung gesetzlicher Pflichten Art. 6 Abs. 1 lit. c DSGVO. Daten werden nur so lange aufbewahrt, wie die Bearbeitung, gesetzliche Pflichten oder die erforderliche Rechtsverteidigung dies verlangen.</p>
 <h2>Betroffenenrechte</h2>
 <p>Nach Maßgabe der gesetzlichen Voraussetzungen bestehen insbesondere Rechte auf Auskunft, Berichtigung, Löschung und Einschränkung. Bei Verarbeitung auf Grundlage berechtigter Interessen besteht ein Widerspruchsrecht nach Art. 21 DSGVO aus Gründen der besonderen Situation. Einwilligungen können jederzeit mit Wirkung für die Zukunft widerrufen werden. Anfragen können an die oben genannte E-Mail-Adresse gerichtet werden.</p>
 <p>Es besteht ein Beschwerderecht nach Art. 77 DSGVO. Für nichtöffentliche Stellen in Bayern ist grundsätzlich das <a href="https://www.lda.bayern.de" target="_blank" rel="noopener noreferrer">Bayerische Landesamt für Datenschutzaufsicht</a> zuständig; besondere medienrechtliche Zuständigkeiten bleiben unberührt.</p>
 <h2>Journalistische Verarbeitung und Medienprivileg</h2>
 <p>Ob und in welchem Umfang § 23 MStV anwendbar ist, richtet sich nach seinen gesetzlichen Voraussetzungen, nicht allein nach der Bezeichnung des Angebots. Die Vorschrift enthält besondere Regeln für journalistische Datenverarbeitung, darunter Datengeheimnis, Datensicherheit, Auskunft, Berichtigung und ergänzende Darstellungen. Sie befreit Hosting, Besucheranalyse oder Kontaktverwaltung nicht pauschal von der DSGVO. Gesetzliche Rechte bleiben unberührt.</p>
 <p><a href="korrektur.html">Korrektur melden</a> · <a href="/">Zurück zum Dashboard</a></p>`,
		legalAddressHTML(a), escAttr(a.LegalEmail), esc(a.LegalEmail), note, analytics, maps, post)
	return legalShell("Datenschutzerklärung", "Datenverarbeitung beim Besuch und bei der Darstellung öffentlicher Profildaten.", body)
}
