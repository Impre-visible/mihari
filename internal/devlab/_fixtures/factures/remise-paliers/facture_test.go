package factures

import "testing"

func TestTotalHT(t *testing.T) {
	f := Facture{Lignes: []Ligne{{"Stylo", 150, 4}, {"Cahier", 320, 2}}}
	if got := f.TotalHT(); got != 1240 {
		t.Errorf("TotalHT = %d, attendu 1240", got)
	}
}

func TestRemise(t *testing.T) {
	f := Facture{Lignes: []Ligne{{"Écran", 60000, 1}}}
	if got := f.Remise(); got != 10 {
		t.Errorf("Remise = %d, attendu 10", got)
	}
}
